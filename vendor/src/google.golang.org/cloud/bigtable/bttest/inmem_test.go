package bttest

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/context"
	btdpb "google.golang.org/cloud/bigtable/internal/data_proto"
	btspb "google.golang.org/cloud/bigtable/internal/service_proto"
	bttdpb "google.golang.org/cloud/bigtable/internal/table_data_proto"
	bttspb "google.golang.org/cloud/bigtable/internal/table_service_proto"
)

func TestConcurrentMutationsAndGC(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.CreateTable(
		ctx,
		&bttspb.CreateTableRequest{Name: "cluster", TableId: "t"}); err != nil {
		t.Fatal(err)
	}
	const name = `cluster/tables/t`
	tbl := s.tables[name]
	req := &bttspb.CreateColumnFamilyRequest{Name: name, ColumnFamilyId: "cf"}
	fam, err := s.CreateColumnFamily(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	fam.GcRule = &bttdpb.GcRule{Rule: &bttdpb.GcRule_MaxNumVersions{MaxNumVersions: 1}}
	if _, err := s.UpdateColumnFamily(ctx, fam); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var ts int64
	ms := func() []*btdpb.Mutation {
		return []*btdpb.Mutation{
			{
				Mutation: &btdpb.Mutation_SetCell_{
					SetCell: &btdpb.Mutation_SetCell{
						FamilyName:      "cf",
						ColumnQualifier: []byte(`col`),
						TimestampMicros: atomic.AddInt64(&ts, 1000),
					},
				},
			},
		}
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				req := &btspb.MutateRowRequest{
					TableName: name,
					RowKey:    []byte(fmt.Sprint(rand.Intn(100))),
					Mutations: ms(),
				}
				s.MutateRow(ctx, req)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			tbl.gc()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Concurrent mutations and GCs haven't completed after 100ms")
	}
}

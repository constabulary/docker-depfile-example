# docker-depfile-example

This is a sample [gb](https://getgb.io/) repository demonstratating the [depfile](https://getgb.io/docs/depfile) feature.

This project requires gb revision d381a0e or later.

## Usage
```
% git clone https://github.com/constabulary/docker-depfile-example
% cd docker-depfile-example
% gb build
% gb build
fetching github.com/docker/docker-credential-helpers (0.3.0)
fetching github.com/gorilla/mux (1.1)
fetching github.com/vbatts/tar-split (0.9.13)
fetching github.com/boltdb/bolt (1.2.1)
fetching github.com/docker/go (1.5.1-1)
fetching github.com/docker/libkv (0.1.0)
fetching github.com/opencontainers/runc (1.0.0-rc1)
fetching github.com/russross/blackfriday (1.4)
fetching github.com/mattn/go-shellwords (1.0.0)
fetching github.com/Sirupsen/logrus (0.10.0)
... fetching and building intensifies ...
```

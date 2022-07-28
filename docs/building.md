# Building the cc-metric-collector

In most cases, a simple `make` in the main folder is enough to get a `cc-metric-collector` binary. It is basically a `go build` but some collectors require additional tasks. There is currently no Golang interface to LIKWID, so it uses `cgo` to create bindings but `cgo` requires the LIKWID header files. Therefore, it checks whether LIKWID is installed and if not it downloads LIKWID and copies the headers.

## System integration

The main configuration settings for system integration are pre-defined in `scripts/cc-metric-collector.config`. The file contains the UNIX user and group used for execution, the PID file location and other settings. Adjust it accordingly and copy it to `/etc/default/cc-metric-collector`

```bash
$ install --mode 644 \
          --owner $CC_USER \
          --group $CC_GROUP \
          scripts/cc-metric-collector.config /etc/default/cc-metric-collector
$ edit /etc/default/cc-metric-collector
```

### SysVinit and similar

If you are using a init system based in `/etc/init.d` daemons, you can use the sample `scripts/cc-metric-collector.init`. It reads the basic configuration from `/etc/default/cc-metric-collector`

```bash
$ install --mode 755 \
          --owner $CC_USER \
          --group $CC_GROUP \
          scripts/cc-metric-collector.init /etc/init.d/cc-metric-collector
```

### Systemd

If you are using `systemd` as init system, you can use the sample systemd service file `scripts/cc-metric-collector.service`, the configuration file `scripts/cc-metric-collector.config`.

```bash
$ install --mode 644  \
          --owner $CC_USER \
          --group $CC_GROUP \
           scripts/cc-metric-collector.service /etc/systemd/system/cc-metric-collector.service
$ systemctl enable cc-metric-collector
```

## RPM

In order to get a RPM packages for cc-metric-collector, just use:

```bash
$ make RPM
```

It uses the RPM SPEC file `scripts/cc-metric-collector.spec` and requires the RPM tools (`rpm` and `rpmspec`) and `git`.

## DEB

In order to get very simple Debian packages for cc-metric-collector, just use:

```bash
$ make DEB
```

It uses the DEB control file `scripts/cc-metric-collector.control` and requires `dpkg-deb`, `awk`, `sed` and `git`. It creates only a binary deb package.

_This option is not well tested and therefore experimental_
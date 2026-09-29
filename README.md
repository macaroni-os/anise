
<p align="center">
  <img src="https://github.com/macaroni-os/macaroni-site/blob/master/site/static/images/logo.png">
</p>

# anise - Container-based Build System and Package manager

Anise is a multi-platform Build System and Package Manager based off from containers - it uses Docker (and others) to build packages.
It has zero dependencies and it is well suitable for "from scratch" environments. It can also version entire rootfs and enables
delivery of OTA-alike updates, making it a perfect fit for the Edge computing era and IoT embedded devices.

It's originally based on [luet-0.17.0](https://github.com/mudler/luet) but later totally rewritten.

It offers a simple in YAML notation to define both packages and rootfs. As it is based on containers, it can be also used to build stages
for Linux From Scratch installations and it can build and track updates for those systems.

It is written entirely in Golang and where used as package manager, it can run in from scratch environment, with zero dependencies.

It has the primary scope to be used in [Macaroni OS](https://www.macaronios.org) and having a good integration with Macaroni OS Mark.

Some notes about `anise` project:

* supply two different binary: `anise` is the client installer normally used by users
  to install/remove packages aka package manager, `anise-build` instead is used for build packages,
  create repos, etc.

* it use the [tar-formers](https://github.com/geaaru/tar-formers/) library
  to manage the tar streams. The same library is now used to unpack the tarball and install packages.
  On installation, it's important to ensure that the unpacked files will be synced to the filesystem
  and so the tar-formers library forces a flush and sync to the filesystem,
  this decreases the installation speed. To speed up this there is a section on luet config that
  could be tuned based on the target system:

  ```yaml
  # ---------------------------------------------
  # Tarball flows configuration section:
  # ---------------------------------------------
  tar_flows:
  #
  #   Enable mutex for parallel creation of directories
  #   in the untar specs. Normally this field must be
  #   set to true, the default value.
  #   mutex4dir: true
  #
  #   Define the max number of open files related
  #   to a single untar operation. Be carefour on
  #   set this option with a big value to avoid
  #   'too open files' errors.
  #   In a normal system this could be also 512.
    max_openfiles: 100
  #
  #   Define the buffer size in KB to use
  #   on create files from tar content.
    copy_buffer_size: 32
  ```
  FWIS, increasing these values to 200/300 max open files and using a buffer of 128 could
  improve performance but this depends on disk speed, hardware, RAM, etc.

* manage the `subsets` feature: Permit to define subsets to choice what files extract from original package.
  This means that we could avoid splitting of a package, for example for Portage metadata, Include files, etc.
  and customize the subsets defined in the original package definition with custom options that could be
  configured from an user at runtime.

* it supports packages `mask` like in Funtoo/Gentoo. This is a mandatory feature to permit different
  versions of the same packages and to avoid only the major version release being installed.
  In a similar way, the same happens when there are two packages supplying the same provides but
  only one is preferred.

## Install

To install `anise`, you can grab a release on the [Release page](https://github.com/macaroni-os/anise/releases) or to install it in your system:

```bash
$> curl https://raw.githubusercontent.com/macaroni-os/anise/refs/heads/geaaru/contrib/config/get_anise_root.sh | sh
$> anise search ...
$> anise install ..
$> anise --help
```

## Build from source

```bash
$ git clone https://github.com/macaroni-os/anise.git
$ cd anise
$ make build
```

## Documentation

Will be soon integrated with Macaroni OS documentation.

## License

Anise is distributed under the terms of GPLv3, check out the LICENSE file.

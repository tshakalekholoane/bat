# `bat`

[![Go Reference](https://pkg.go.dev/badge/tshaka.dev/x/bat.svg)](https://pkg.go.dev/tshaka.dev/x/bat)
![Continuous Integration](https://github.com/tshakalekholoane/bat/actions/workflows/ci.yaml/badge.svg)

```                                     bat(1)
NAME
    bat -- battery management utility for Linux laptops 

    bat [-hv]
        [--help] [--version]
        <command> [<arg>]

OPTIONS
    -h, --help      
        Print this help document.

    -v, --version
        Display version information and exit.

COMMANDS
    capacity
        Print the current battery level.

    health
        Print the battery health status.

    persist   
        Persist the current threshold between restarts.

    reset    
        Undoes the persistence setting of the charging threshold between 
        restarts.

    status
        Print the charging status.

    threshold num
        Print the current charging threshold limit.

        If num is specified--which should be a value between 1 and 100--this
        will set a new charging threshold limit.
```

## About

The goal is to replicate the functionality of the [ASUS Battery Health Charging](https://www.asus.com/us/support/FAQ/1032726/) utility for ASUS laptops on Windows which aims to prolong the battery's life-span <a href="https://electrek.co/2017/09/01/tesla-battery-expert-recommends-daily-battery-pack-charging/"><sup>1</sup></a> <a href="https://batteryuniversity.com/learn/article/how_to_prolong_lithium_based_batteries"><sup>2</sup></a>.

## Disclaimer

This has been reported to only work with some ASUS and [Lenovo ThinkPad](https://github.com/tshakalekholoane/bat/discussions/23) laptops only. For Dell systems, see [smbios-utils](https://github.com/dell/libsmbios), particularly the `smbios-battery-ctl` command, or install it using your package manager. For other manufacturers there is also [TLP](https://linrunner.de/tlp/).

There have also been some [problems setting the charging threshold inside of a virtual machine](https://github.com/tshakalekholoane/bat/issues/3#issuecomment-858581495).

## Installation

Precompiled binaries (Linux x86-64) are available from the [GitHub releases page](https://github.com/tshakalekholoane/bat/releases), the latest of which can be downloaded from [here](https://github.com/tshakalekholoane/bat/releases/download/0.13.1/bat).

After downloading the binary, give it permission to execute on your system by running the following command. For example, assuming the binary is located in the user's Downloads folder:

```shell
chmod +x $HOME/Downloads/bat
```

Alternatively, the application can be build from source by running the following command in the root directory of this repository. This requires a working version of [Make](https://www.gnu.org/software/make/) and [Go](https://golang.org/).

```shell
make build
```

**Tip**: Create a symbolic link of the resulting binary in a directory that is in the `$PATH` environment variable such as `/usr/local/bin/`. This will allow any user to execute the program from anywhere on the system.

```shell
ln -s $HOME/Downloads/bat /usr/local/bin/bat
```

You can also rename the binary to something else if another program with the same name already exists i.e. [bat](https://github.com/sharkdp/bat).

## Examples

```shell
# Print the current battery charging threshold.
bat threshold

# Set a new charging threshold, say 80% (requires superuser 
# permissions).
sudo bat threshold 80

# Persist the current charging threshold setting between restarts 
# (requires superuser permissions).
sudo bat persist
```

## Requirements

Linux kernel version later than 5.4-rc1 which is the [earliest version to expose the battery charging threshold variable](https://github.com/torvalds/linux/commit/7973353e92ee1e7ca3b2eb361a4b7cb66c92abee).

To persist the threshold setting between restarts, the application relies on [systemd](https://systemd.io/), particularly a version later than 244, and a POSIX shell `sh`, which are bundled with most Linux distributions.

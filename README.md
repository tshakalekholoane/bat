# `bat`

```
                                      bat
NAME
    bat - battery management utility for Linux laptops

SYNOPSIS
    bat [OPTION]

DESCRIPTION
    -c, --capacity      print the current battery level
    -h, --help          print this help document
    -p, --persist       persist the current charging threshold setting between
                        restarts (requires superuser permissions)
    -r, --reset         prevents the charging threshold from persisting between
                        restarts
    -s, --status        print charging status
    -t, --threshold     print the current charging threshold limit
                        specify a value between 1 and 100 to set a new threshold
                        (the latter requires superuser permissions)
                        e.g. bat --threshold 80
```

## About

The goal is to replicate the functionality of the [ASUS Battery Health Charging](https://www.asus.com/us/support/FAQ/1032726/) utility for ASUS laptops on Windows which aims prolong the battery's life-span <a href="https://electrek.co/2017/09/01/tesla-battery-expert-recommends-daily-battery-pack-charging/"><sup>1</sup></a> <a href="https://batteryuniversity.com/learn/article/how_to_prolong_lithium_based_batteries"><sup>2</sup></a>.

## Disclaimer

This has only shown to work on ASUS laptops. For Dell systems, see [smbios-utils](https://github.com/dell/libsmbios), particularly the `smbios-battery-ctl` command, or install it using your package manager. For other manufacturers there is also [TLP](https://linrunner.de/tlp/).

## Installation

Precompiled binaries (Linux x86-64) are available from the [GitHub releases page](https://github.com/leveson/bat/releases), the latest of which can be downloaded from [here](https://github.com/leveson/bat/releases/download/0.5/bat).

After downloading the binary, give it permission to execute on your system by running the following command. For example, assuming the binary is located in the user's Downloads folder:

```shell
$ chmod +x $HOME/Downloads/bat
```

Alternatively, the application can be build from source by running the following [Go](https://golang.org/) command.

```shell
$ go build bat.go
```

**Tip**: Place the resulting binary in a directory that is in the `$PATH` environment variable such as `/usr/local/bin/`. This will allow the user to execute the program from anywhere on their system.

**Another tip**: Rename the binary to something else if another program with the same name already exists on your system i.e. [bat](https://github.com/sharkdp/bat).

## Examples

```shell
# Print the current battery charging threshold.
$ bat --threshold

# Set a new charging threshold, say 80%.
# (requires superuser permissions).
$ sudo bat --threshold 80

# Persist the current charging threshold setting between restarts
# (requires superuser permissions).
$ sudo bat --persist
```

## Requirements

Linux kernel version later than 5.4 which is the [earliest version to expose the battery charging threshold variable](https://github.com/torvalds/linux/commit/7973353e92ee1e7ca3b2eb361a4b7cb66c92abee).

To persist the threshold setting between restarts, the application relies on [systemd](https://systemd.io/), particularly a version later than 244, and [Bash](https://www.gnu.org/software/bash/) which are bundled with most Linux distributions. 

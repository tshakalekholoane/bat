# `bat`

```
                                      bat
                                      
NAME
    bat - battery management utility for Linux laptops 

SYNOPSIS
    bat [OPTION]
    
DESCRIPTION
    -c, --capacity      print current battery level
    -h, --help          print this help document
    -p, --persist       persist the current threshold setting between restarts
                        (requires sudo permissions)
    -t, --threshold     print the current charging threshold limit
                        append a value between 1 and 100 to set a new threshold
    -s, --status        print charging status
```

## About

Aims to replicate the functionality of the [ASUS Battery Health Charging](https://www.asus.com/us/support/FAQ/1032726/) utility for ASUS laptops on Windows which aims to prolong the battery's life-span <a href="https://electrek.co/2017/09/01/tesla-battery-expert-recommends-daily-battery-pack-charging/"><sup>1</sup></a> <a href="https://batteryuniversity.com/learn/article/how_to_prolong_lithium_based_batteries"><sup>2</sup></a>.

## Installation

Precompiled binaries (Linux x86-64) are available from the [GitHub releases page](https://github.com/leveson/bat/releases).

Alternatively, one could build the binary oneself by running the following [Go](https://golang.org/) command,
```shell
$ go build bat.go
```
and placing the resulting binary in a directory that is in their `$PATH` such as `/usr/local/bin`.

## Requirements

To persist threshold settings between restarts, the application relies on [Bash](https://www.gnu.org/software/bash/) and [systemd](https://systemd.io/) which are bundled with most Linux distributions.

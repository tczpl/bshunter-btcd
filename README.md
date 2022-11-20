BSHunter-BTCD
====

BSHunter-BTCD is the modified Bitcoin client ([BTCD](https://github.com/btcsuite/btcd)) in BSHunter.

This is an anonymous repository for ICSE 2023 Response, in order to show our script extraction in transactions and defect tracing in Bitcoin VM. We will make more details in the future.

As for the other source codes of benchmark, please refer to [UnsafeBTC.com](https://unsafebtc.com/#/app/sourcecode).

## Requirements

[Go](http://golang.org) 1.14 or newer.

## Build

```bash
$ ./build.sh
```

## Run

```bash
$ # Sync the Bitcoin blockchain
$ ./btcd --tx-index
$ # Select the features by changing the "Run()" in bshunter.go
$ # Extract the scripts
$ ./btcd --bshunter --tx-index
```

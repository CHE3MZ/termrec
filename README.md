# termrec

Private terminal recorder. Record your terminal, play it back, render animated GIFs. No upload, no network.

## Install

```sh
go install github.com/CHE3MZ/termrec/cmd/termrec@latest
# or
task build
./dist/termrec --help
```

Requires Go >= 1.26.

## Use

```sh
termrec init
termrec config
termrec record demo -c config.yml -d "powershell.exe"
termrec play demo -r -s 1.5
termrec render demo -o demo.gif -q 90 -s 2
```

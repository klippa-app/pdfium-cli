# pdfium-cli

[![Build Status][build-status]][build-url]

[build-status]:https://github.com/klippa-app/pdfium-cli/workflows/Go/badge.svg

[build-url]:https://github.com/klippa-app/go-pdfium/actions

:rocket: *Easy to use PDF CLI tool powered by [PDFium](http://pdfium.org) and [go-pdfium](https://github.com/klippa-app/go-pdfium)* :rocket:

## Features

* Exploding PDFs into one PDF file per page
* Rendering PDFs in different formats

## PDFium

This project uses the PDFium C++ library by Google (https://pdfium.googlesource.com/pdfium/) to process the PDF
documents.

## Prerequisites

To use this CLI tool, you will need the actual PDFium library to run it.

### Get the PDFium library

You can try to compile PDFium yourself, but you can also use pre-compiled binaries, for example
from: https://github.com/bblanchon/pdfium-binaries/releases

If you use a pre-compiled library, make sure to extract it somewhere logical, for example /opt/pdfium.

### Configure LD_LIBRARY_PATH

Extend your library path by running:

`export LD_LIBRARY_PATH={path}/lib`

Replace `{path}` with the path you extracted/compiled pdfium in.

You can do this globally or just in your editor.

this can globally be done on ubuntu by editing `~/.profile`
and adding the line in this file. reloading for bash can be done by relogging or running `source ~/.profile` can be used
to test the change for a terminal

## Getting started

### From binary

Download the binary from the latest release for your platform and save it as `pdfium`.

You can also use the `install` tool for this:

```bash
sudo install pdfium-linux-x64 /usr/local/bin/pdfium
```

### From source

Make sure you have a working Go development environment.

Clone the repository:

```bash
git clone https://github.com/klippa-app/pdfium-cli.git
```

Move into the directory:

```bash
cd pdfium-cli
```

Run the command:

```bash
go run main.go
```

Or to compile and run pdfium-cli:

```bash
go build -o pdfium main.go
./pdfium -h
```

Output:

```text
pdfium-cli is a CLI tool that allows you to use pdfium from the CLI

Usage:
  pdfium [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  explode     Explode a PDF into multiple PDFs
  help        Help about any command
  render      Render a PDF into images

Flags:
  -h, --help   help for pdfium

Use "pdfium [command] --help" for more information about a command.
```

## About Klippa

Founded in 2015, [Klippa](https://www.klippa.com/en)'s goal is to digitize & automate administrative processes with
modern technologies. We help clients enhance the effectiveness of their organization by using machine learning and OCR.
Since 2015, more than a thousand happy clients have used Klippa's software solutions. Klippa currently has an
international team of 50 people, with offices in Groningen, Amsterdam and Brasov.

## License

The MIT License (MIT)

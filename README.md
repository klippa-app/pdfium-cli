# pdfium-cli

[![Build Status][build-status]][build-url]

[build-status]:https://github.com/klippa-app/pdfium-cli/workflows/Go/badge.svg

[build-url]:https://github.com/klippa-app/go-pdfium/actions

:rocket: *Easy to use PDF CLI tool powered by [PDFium](http://pdfium.org) and [go-pdfium](https://github.com/klippa-app/go-pdfium)* :rocket:

## Features

* Get information of a PDF
* Merge multiple PDFs into a single PDF
* Extracting text from PDFs
* Extracting images from PDFs
* Exploding PDFs into one PDF file per page
* Rendering PDFs in different formats

## PDFium & Wazero

This project uses the PDFium C++ library by Google (https://pdfium.googlesource.com/pdfium/) to process the PDF
documents.

We use a Webassembly version of PDFium that is compiled with [Emscripten](https://emscripten.org/) and runs in the [Wazero Go](https://github.com/tetratelabs/wazero) runtime.

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
  images      Extract the images of a PDF
  info        Get the information of a PDF
  merge       Merge multiple PDFs into a single PDF
  render      Render a PDF into images
  text        Get the text of a PDF

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

Wazero and PDFium come with the `Apache License 2.0` license
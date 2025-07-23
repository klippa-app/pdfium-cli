# pdfium-cli

[![Build Status][build-status]][build-url]
[![Release Status][release-status]][build-url]

[build-status]:https://github.com/klippa-app/pdfium-cli/workflows/Go%20tests/badge.svg

[build-url]:https://github.com/klippa-app/pdfium-cli/actions

[release-status]:https://github.com/klippa-app/pdfium-cli/workflows/Release%20binaries/badge.svg

:rocket: *Easy to use PDF CLI tool powered by [PDFium](http://pdfium.org) and [go-pdfium](https://github.com/klippa-app/go-pdfium)* :rocket:

## Features

* Get information of a PDF
* Merge multiple PDFs into a single PDF
* Exploding PDFs into one PDF file per page
* Rendering PDFs in JPG and PNG
* Extracting text from PDFs
* Extracting images from PDFs
* Extracting attachments from PDFs
* Extracting thumbnails from PDFs
* Extracting JavaScripts from PDFs
* Flattening PDFs
* Piping input through stdin when the input is one file (use filename `-`)
* Piping output through stdout when the output is one file (use filename `-`)

## PDFium & Wazero

This project uses the PDFium C++ library by Google (https://pdfium.googlesource.com/pdfium/) to process the PDF
documents.

We use a Webassembly version of PDFium that is compiled with [Emscripten](https://emscripten.org/) and runs in the [Wazero Go](https://github.com/tetratelabs/wazero) runtime.

## Getting started

### From binary

Download the binary from the latest release for your platform and save it as `pdfium`.

You can also use the `install` tool for this:

```bash
sudo install pdfium-webassembly-linux-x64 /usr/local/bin/pdfium
```

#### Release types

The following release types are available:

- Linux
  - WebAssembly (amd64 + arm64)
  - Native (amd64)
  - Native + MUSL (amd64)
- MacOS
  - WebAssembly (amd64 + arm64)
  - Native (amd64 + arm64)
- Windows
  - WebAssembly (amd64)
  - Native (amd64)


**WebAssembly**: this is a single binary that includes everything that you need to run pdfium-cli, but is a lot slower
than native due to the WebAssembly runtime. Most useful if speed is not a concern and easy distribution is more 
important.

**Native**: A native build that requires [pdfium](https://github.com/bblanchon/pdfium-binaries) and
[libjpeg-turbo](https://libjpeg-turbo.org/) to be available on your system.

**Native + MUSL**: Same as native but built with MUSL so that it does not require a system libc which allows it to be
used in Alpine Docker containers.

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
  attachments Extract the attachments of a PDF
  completion  Generate the autocompletion script for the specified shell
  explode     Explode a PDF into multiple PDFs
  flatten     Flatten a PDF
  help        Help about any command
  images      Extract the images of a PDF
  info        Get the information of a PDF
  javascripts Extract the javascripts of a PDF
  merge       Merge multiple PDFs into a single PDF
  render      Render a PDF into images
  text        Get the text of a PDF
  thumbnails  Extract the attachments of a PDF


Flags:
  -h, --help   help for pdfium

Use "pdfium [command] --help" for more information about a command.
```

The following build tags are available to control different build types:

 - pdfium_cli_use_cgo: whether to compile the native CGO version (faster, but requires [pdfium](https://github.com/bblanchon/pdfium-binaries) to be installed).
 - pdfium_experimental: whether to enable experimental features of pdfium in the build.
 - pdfium_use_turbojpeg: whether to enable [libjpeg-turbo](https://libjpeg-turbo.org/) support, which speeds up jpeg compression a lot compared to the default jpeg encoding in Go.

## About Klippa

Founded in 2015, [Klippa](https://www.klippa.com/en)'s goal is to digitize & automate administrative processes with
modern technologies. We help clients enhance the effectiveness of their organization by using machine learning and OCR.
Since 2015, more than a thousand happy clients have used Klippa's software solutions. Klippa currently has an
international team of 50 people, with offices in Groningen, Amsterdam and Brasov.

## License

The MIT License (MIT)

Wazero and PDFium come with the `Apache License 2.0` license

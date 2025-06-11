## Installation

Download the latest sup binary from https://github.com/rubiojr/sup/releases.

### From source

#### Prerequisites

- Go 1.24 or later

#### Build from source

```bash
git clone https://github.com/rubiojr/sup
cd sup
make build
```

Or build manually:

```bash
go build -o sup ./cmd/sup
```

#### With go install

```bash
go install github.com/rubiojr/sup/cmd/sup@latest
```

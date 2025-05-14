## golangci-lint

### 1. Install golangci-lint

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.1.6

golangci-lint --version
```

See https://golangci-lint.run/welcome/install/#local-installation

### 2. Lint

```bash
golangci-lint run

# You can choose which directories or files to analyze:
golangci-lint run dir1 dir2/...
golangci-lint run file1.go
```



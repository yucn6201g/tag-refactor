# ビルドの手順

## 準備

| 項目 | 内容 |
|---|---|
| Go | 最新の安定版。`go.mod` の `go` の行は `1.26.0` で、CI は stable と oldstable で動かす。手元は `GOTOOLCHAIN=local` で動かす |
| 依存の入手 | `go mod download`。`go.sum` で確かめられる。ローカルのサービスや設定ファイルは要らない |
| cgo | 要らない。配るバイナリは `CGO_ENABLED=0`。`-race` のときだけ cgo が要る(手元の Windows には C コンパイラーがなく、CI の `race` ジョブで動かす) |
| git | `iac --write` の実行時に使う。ビルドには要らない |

## ビルド

```bash
go build -mod=readonly -trimpath -o tag-refactor ./cmd/tag-refactor
```

配るものと同じ条件で、6つの組み合わせをビルドする。

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath -o tag-refactor-linux-amd64 ./cmd/tag-refactor
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -mod=readonly -trimpath -o tag-refactor-linux-arm64 ./cmd/tag-refactor
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -mod=readonly -trimpath -o tag-refactor-darwin-amd64 ./cmd/tag-refactor
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -mod=readonly -trimpath -o tag-refactor-darwin-arm64 ./cmd/tag-refactor
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=readonly -trimpath -o tag-refactor-windows-amd64.exe ./cmd/tag-refactor
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -mod=readonly -trimpath -o tag-refactor-windows-arm64.exe ./cmd/tag-refactor
```

## ビルドの確かめ

```bash
gofmt -l cmd internal scripts
go vet ./...
go vet -tags integration ./...
golangci-lint run
golangci-lint run --build-tags integration
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

`gofmt -l` は何も出さないこと。`golangci-lint` は既定の linter だけを使う(チームの進め方)。

## うまくいかないとき

| 症状 | 原因と対応 |
|---|---|
| `go: updates to go.mod needed` | `-mod=readonly` のため。`go mod tidy` の差分を確かめてコミットする(CI の `modcheck` と同じ) |
| `-race requires cgo` | 手元に C コンパイラーがない。CI の `race` ジョブに任せる |
| `govulncheck` が標準ライブラリを指摘する | 手元の Go が古い。指摘の直る版に Go を上げる。外部のモジュールの指摘は依存の更新で直す |
| `toolchain not available` | `GOTOOLCHAIN=local` で、手元の Go が `go` の行より古い。Go を上げる |

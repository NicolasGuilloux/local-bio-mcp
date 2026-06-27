{ pkgs, lib, config, ... }:

{
  # https://devenv.sh/basics/
  env.GREET = "local.bio MCP & CLI dev shell";
  env.CGO_ENABLED = "0";

  # https://devenv.sh/packages/
  packages = with pkgs; [
    git
    go-tools          # staticcheck, etc.
    golangci-lint
    gofumpt
    delve             # debugger
    jq
    curl
    docker-client
  ];

  # https://devenv.sh/languages/
  languages.go = {
    enable = true;
    package = pkgs.go;
  };

  # https://devenv.sh/scripts/
  scripts.build.exec = ''
    set -euo pipefail
    mkdir -p "$DEVENV_ROOT/bin"
    go build -trimpath -ldflags "-s -w" -o "$DEVENV_ROOT/bin/localbio" "$DEVENV_ROOT/cmd/localbio"
    echo "built -> bin/localbio"
  '';

  scripts.run.exec = ''go run "$DEVENV_ROOT/cmd/localbio" "$@"'';

  scripts.test.exec = ''go test ./... "$@"'';

  scripts.lint.exec = ''
    set -euo pipefail
    gofumpt -l -w .
    go vet ./...
    golangci-lint run ./... || true
  '';

  scripts.tidy.exec = ''go mod tidy'';

  # MCP server helpers
  scripts.mcp-stdio.exec = ''go run "$DEVENV_ROOT/cmd/localbio" mcp'';
  scripts.mcp-http.exec = ''go run "$DEVENV_ROOT/cmd/localbio" mcp http "''${1:-:8080}"'';

  # Docker image
  scripts.docker-build.exec = ''
    docker build -t local-bio-mcp:dev "$DEVENV_ROOT"
  '';

  # CDP exploration helper (DEV ONLY — never used by the CLI itself)
  scripts.cdp.exec = ''
    : "''${CDP_URL:=http://localhost:9222}"
    echo "Chrome DevTools endpoint: $CDP_URL"
    curl -s "$CDP_URL/json/version" | jq .
  '';

  enterShell = ''
    echo "$GREET"
    go version
    echo "Scripts: build | run | test | lint | tidy | mcp-stdio | mcp-http | docker-build"
  '';

  # https://devenv.sh/tests/
  enterTest = ''
    go build ./...
    go test ./...
  '';
}

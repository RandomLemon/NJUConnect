# shell.nix
{
  pkgs ? import <nixpkgs> { },
}:

pkgs.mkShell {
  name = "easierconnect";

  buildInputs = with pkgs; [
    go
    gopls
  ];
  shellHook = ''
    export GOBIN=$PWD/.go/bin
    export GOPATH=$PWD/.go
    export GOCACHE=$PWD/.cache/go-build
    export PATH=$GOBIN:$PATH

    mkdir -p $GOPATH $GOBIN

    go env -w GOPROXY="https://goproxy.cn,direct"
    echo "Go development environment ready!"
    echo "Go version: $(go version)"
  '';
}


#!/bin/sh -e

# Copied w/ love from the excellent hypnoglow/helm-s3

if [ -n "${HELM_PUSH_PLUGIN_NO_INSTALL_HOOK}" ]; then
    echo "Development mode: not downloading versioned release."
    exit 0
fi

# initARch and initOS copied w/ love from https://github.com/technosophos/helm-template

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="armv7";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
  esac
}

# initOS discovers the operating system for this system.
initOS() {
  OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

  case "$OS" in
    # Msys support
    msys*) OS='windows';;
    # Minimalist GNU for Windows
    mingw*) OS='windows';;
  esac
}

initArch
initOS

VER=$(awk '/version:/{gsub(/\"/,"", $2); print $2}' plugin.yaml)
version=v${VER}
#version="$(curl -s https://api.github.com/repos/mumoshu/helm-x/releases/latest | awk '/\"tag_name\":/{gsub( /[,\"]/,"", $2); print $2}')"
echo "Downloading and installing helm-x ${version} ..."

VER=${version/v/}
url="https://github.com/mumoshu/helm-x/releases/download/${version}/helm-x_${VER}_${OS}_${ARCH}.tar.gz"

echo $url

cd $HELM_PLUGIN_DIR
mkdir -p "bin"
mkdir -p "releases/${version}"

# Download with curl if possible.
if [ -x "$(which curl 2>/dev/null)" ]; then
    curl -sSL "${url}" -o "releases/${version}.tgz"
else
    wget -q "${url}" -O "releases/${version}.tgz"
fi

find releases
tar xzf "releases/${version}.tgz" -C "releases/${version}"
mv "releases/${version}/helm-x" "bin/helm-x" || \
    mv "releases/${version}/helm-x.exe" "bin/helm-x"
rm -rf releases

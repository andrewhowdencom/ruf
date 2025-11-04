# Script to bootstrap an environment (e.g. Google Jules) to something that can be used for development.
# Safeties
set -exuo pipefail

# Install the more recent version of Golang
cd $(mktemp -d)
curl --output \
  --location \
  https://go.dev/dl/go1.25.3.linux-amd64.tar.gz

sudo rm -rf /usr/local/go && \
  sudo tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile

go version

# Install Taskfile
curl -1sLf 'https://dl.cloudsmith.io/public/task/task/setup.deb.sh' | sudo -E bash

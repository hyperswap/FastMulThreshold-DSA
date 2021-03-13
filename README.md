# Introduction
Fast Multiparty Threshold DSA is a distributed key generation and distributed signature service that can serve as a distributed custodial solution.

*Note : dcrm-walletService is considered beta software. We make no warranties or guarantees of its security or stability.*

# Install the Docker version
## 1. Install Docker. This depends on your platform, on Ubuntu this works:
```
sudo apt update
sudo apt install docker.io
```
## 2. Download the Docker image and create and run the container:
- bootnode
```
docker run -d --name bootnode --network host --restart always -v /var/lib/docker/bootnode:/bootnode anyswap/bootnode --addr :12340
```
- gdcrm
```
docker run -d --name gdcrm --network host --restart always -v /var/lib/docker/gdcrm:/gdcrm anyswap/gdcrm --bootnodes "enode://ip@port" --port 12345 --rpcport 23456
```
- gdcrm-client
```
docker exec gdcrm gdcrm-client --cmd ACCEPTREQADDR --url http://127.0.0.1:23456 --keystore keystore --passwd "123456" --key 0x...
```

# Install the Source version
# Prerequisites
1. VPS server with 1 CPU and 2G mem
2. Static public IP
3. Golang ^1.12

# Setting Up
## Clone The Repository
To get started, launch your terminal and download the latest version of the SDK.
```
mkdir -p $GOPATH/src/github.com/anyswap

cd $GOPATH/src/github.com/anyswap

git clone https://github.com/anyswap/FastMulThreshold-DSA.git
```
## Build
Next compile the code.  Make sure you are in FastMulThreshold-DSA directory.
```
cd FastMulThreshold-DSA && make
```

## Run
First generate the node key: 
```
./bin/cmd/gdcrm --genkey node1.key
```

then run the dcrm node 7x24 in the background:
```
nohup ./bin/cmd/gdcrm --nodekey node1.key &
```
The `gdcrm` will provide rpc service, the default RPC port is port 4449.

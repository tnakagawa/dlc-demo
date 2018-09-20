#!/bin/bash

STOP_SH="./stop.sh"

if [ -e $STOP_SH ]; then
    $STOP_SH
    sleep 3
    rm -rf $STOP_SH
fi

# change the setting
B_DIR="$HOME/bitcoin/src"

BD="$B_DIR/bitcoind"

if [ ! -e $BD ]; then
    echo "file not found. $BD"
    exit 1
fi

BC=$B_DIR/bitcoin-cli

if [ ! -e $BC ]; then
    echo "file not found. $BC"
    exit 1
fi

DATA_DIR="data"

if [ ! -d "$DATA_DIR" ]; then
    rm -rf $DATA_DIR
    mkdir $DATA_DIR
fi

if [ ! -d "$DATA_DIR/dlc" ]; then
    rm -rf "$DATA_DIR/dlc"
    mkdir "$DATA_DIR/dlc"
    cat > "$DATA_DIR/dlc/bitcoin.conf" << EOF
rpcuser=user
rpcpassword=pass
regtest=1
txindex=1
keypool=10
deamon=1
listen=1
EOF
    echo "setup dlc"
fi

PWD=`pwd`
BDD="$BD -datadir=$PWD/$DATA_DIR/dlc"
BCD="$BC -datadir=$PWD/$DATA_DIR/dlc"

$BDD &

echo -n "bitcoind starting"

LDW=1
while [ "${LDW}" = "1" ]
do
    LDW=0
    $BCD getwalletinfo > /dev/null 2>&1 || LDW=1
    if [ "${LDW}" = "1" ]; then
        echo -n "."
        sleep 1
    fi
done
echo ""

echo "alias dlcdemo=\"$BCD\""

cat > $STOP_SH << _EOF_
#!/bin/bash
echo "bitcoind stop"
$BCD stop

rm -rf $STOP_SH

_EOF_

chmod 755 $STOP_SH

demo/demo

exit 0

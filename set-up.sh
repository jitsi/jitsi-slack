
set -e

. env.sh

echo "Installing Go 1.11"
wget -q -O - https://raw.githubusercontent.com/canha/golang-tools-install-script/master/goinstall.sh \
| bash -s -- --version 1.11

source /root/.bashrc


sleep 5

echo "Setting up DynamoDB table"

aws dynamodb create-table --cli-input-json file://$TOKEN_TABLE_CONFIG --region $DYNAMO_REGION 

aws dynamodb create-table --cli-input-json file://$SERVER_TABLE_CONFIG --region $DYNAMO_REGION


sleep 5

echo "Done"
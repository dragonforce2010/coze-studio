#!/bin/bash
# init env
set -a; source .env; set +a;

# download opencoze 
wget -O- https://skip-jump.tos-cn-beijing.volces.com/opencoze-0.2.2.tgz | tar -zxf - -C .

# copy install.sh
cp opencoze/install.sh .
cp opencoze/uninstall.sh .

# install opencoze
bash install.sh
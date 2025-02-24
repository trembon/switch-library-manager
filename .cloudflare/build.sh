#!/bin/sh

git clone https://github.com/blawar/nut
cd nut

pip install -r requirements.txt
git clone --depth=1 https://github.com/blawar/titledb

nut.py --import-region US --language en

mkdir ../titles_output

cp titledb/titles.json ../titles_output/titles.json
cp titledb/cnmts.json ../titles_output/cnmts.json
cp titledb/versions.json ../titles_output/versions.json
cp titledb/versions.txt ../titles_output/versions.txt
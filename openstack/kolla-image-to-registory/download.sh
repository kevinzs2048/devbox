#!/bin/bash

filename="download_image_list"

tag=${1:-"queens-20180207"}

while read line 
do 
#sudo docker pull $line:$tag
uuid=`sudo docker images |grep $line | grep $tag | awk '{print $3}'`
sudo docker tag $uuid localhost:5000/$line:$tag
sudo docker push localhost:5000/$line:$tag
done <$filename


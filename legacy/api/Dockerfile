FROM node:16

WORKDIR /home/app

USER root

RUN apt-get -y update

RUN apt-get install -y ffmpeg

COPY . ./

RUN npm install 


RUN npm run build


# CMD ["node","/home/app/dist/src/index.js"]



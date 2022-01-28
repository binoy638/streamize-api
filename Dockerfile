FROM node:latest

WORKDIR /home/app

USER root

RUN apt-get -y update

RUN apt-get install -y ffmpeg

COPY package*.json ./

RUN npm install 

COPY . ./


RUN npm run build


# CMD ["node","/home/app/dist/src/index.js"]



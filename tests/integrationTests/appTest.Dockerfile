FROM node:16-alpine

RUN mkdir /app
WORKDIR /app

COPY package*.json ./

RUN npm install

EXPOSE 3000

COPY index.js ./
ENTRYPOINT [ "node", "index.js" ]

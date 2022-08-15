
# Streamize - Stream Torrent Videos Online (Server)

Streamize is a web app which allows you to download torrents in a remote server and stream them right from your browser.

## Run Locally with Docker

Clone the project

```bash
  git clone https://github.com/binoy638/streamize-client
```

Go to the project directory

```bash
  cd streamize-api
```

Run the project using docker-compose

```bash
  docker-compose -f docker-compose.dev.yml up --build
```

## Run Locally without Docker

Make sure you have ffmpeg, rabbitMQ and mongoDB installed locally before following the below steps.

Clone the project

```bash
  git clone https://github.com/binoy638/streamize-client
```

Go to the project directory

```bash
  cd streamize-api
```

Install dependencies

```bash
  npm install
```

Start the server

```bash
  npm run dev
```

## Environment Variables

To run this project, you will need to add the following environment variables to your .env.local file

`JWT_SECRET`

`COOKIE_SECRET`

`ADMIN_USER`

`ADMIN_PASSWORD`

`ORIGIN_URL` (Frontend client's url)

`GUEST_USER` (Optional, use it if you need guest user)

`GUEST_PASSWORD` (Optional, use it if you need guest user)

`GUEST_STORAGE`  (Optional, provide the storage space in bytes)

## 🔗 Links

[Live Demo](https://streamize.backendev.com/)

[Client Repo](https://github.com/binoy638/streamize-client)

## Disclaimer

Streamize is developed only for educational purposes and is not intended for commercial use.

## License

[MIT](https://choosealicense.com/licenses/mit/)

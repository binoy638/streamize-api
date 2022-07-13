# Streamize - Download and stream torrents online

## Important Note
You need to have docker installed to run this project. 

## Run development docker container 

Run the following command to launch the project inside a docker container

```bash
  docker-compose -f docker-compose.dev.yml up --build
```

## Run production docker container

Run the following command build & run a docker container for production

```bash
  docker-compose up
```

### Optional

To setup [husky](https://www.npmjs.com/package/husky) & [lint-staged](https://www.npmjs.com/package/lint-staged) for pre-commit hook run

```bash
  npm run install:husky
```

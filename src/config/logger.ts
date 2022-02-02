import { createLogger, format, transports } from 'winston';

const { combine, printf, json, colorize, errors } = format;

const prodLogger = () => {
  return createLogger({
    level: 'info',
    format: combine(format.timestamp(), errors({ stack: true }), json()),
    defaultMeta: { service: 'user-service' },
    transports: [new transports.Console()],
  });
};

const devLogger = () => {
  const logFormat = printf(({ level, message, timestamp, stack }) => {
    return `${timestamp} ${level}: ${stack || message}`;
  });

  return createLogger({
    level: 'debug',
    format: combine(
      colorize(),
      format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
      errors({ stack: true }),
      format.splat(),
      format.json(),
      logFormat
    ),
    transports: [new transports.Console()],
  });
};

const logger = process.env.NODE_ENV === 'development' ? devLogger() : prodLogger();

export default logger;

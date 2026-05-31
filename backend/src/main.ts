// MUST stay first — the OTel SDK patches http/express/typeorm at
// require time, so any module imported before this line is invisible
// to the tracer. No-op when OTEL_EXPORTER_OTLP_ENDPOINT is unset.
import './tracing';

import { NestFactory } from '@nestjs/core';
import { BadRequestException, Logger, ValidationPipe } from '@nestjs/common';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { AppModule } from './app.module';
import { AllExceptionsFilter } from './common/all-exceptions.filter';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { rawBody: true });

  // Validation rejections used to fail silently in container logs — clients
  // see HTTP 400 and the user sees "submit failed" with no record on the
  // server side. Log the offending fields so the next time an Android
  // os_info-style overflow happens, ops can spot it in the access log.
  const validationLogger = new Logger('Validation');
  app.useGlobalPipes(new ValidationPipe({
    whitelist: true,
    forbidNonWhitelisted: true,
    transform: true,
    exceptionFactory: (errors) => {
      const summary = errors.map(e => {
        const constraints = Object.values(e.constraints ?? {}).join(', ');
        return `${e.property}: ${constraints}`;
      }).join('; ');
      validationLogger.warn(`Validation failed → ${summary}`);
      return new BadRequestException(
        errors.flatMap(e => Object.values(e.constraints ?? {})),
      );
    },
  }));

  // Single global filter converts every thrown error to the canonical
  // { error, code, request_id } envelope.  Mirrors the Go /rewrite
  // service shape so clients write one decoder.
  app.useGlobalFilters(new AllExceptionsFilter());

  app.enableCors();

  const config = new DocumentBuilder()
    .setTitle('DraftRight API')
    .setDescription('AI-powered text rewriting backend')
    .setVersion('1.0')
    .addBearerAuth()
    .build();
  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api/docs', app, document);

  const port = process.env.PORT || 3000;
  await app.listen(port);
  console.log(`DraftRight API running on port ${port}`);
}
bootstrap();

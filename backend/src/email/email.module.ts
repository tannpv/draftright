import { Module, Global } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { EmailService } from './email.service';
import { EmailWebhookController } from './email-webhook.controller';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { EmailLog } from './entities/email-log.entity';
import { EmailTemplate } from './entities/email-template.entity';
import { EmailSuppression } from './entities/email-suppression.entity';

@Global()
@Module({
  imports: [TypeOrmModule.forFeature([AppSettings, EmailLog, EmailTemplate, EmailSuppression])],
  controllers: [EmailWebhookController],
  providers: [EmailService],
  exports: [EmailService, TypeOrmModule],
})
export class EmailModule {}

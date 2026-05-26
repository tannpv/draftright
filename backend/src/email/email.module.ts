import { Module, Global } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { EmailService } from './email.service';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { EmailLog } from './entities/email-log.entity';
import { EmailTemplate } from './entities/email-template.entity';

@Global()
@Module({
  imports: [TypeOrmModule.forFeature([AppSettings, EmailLog, EmailTemplate])],
  providers: [EmailService],
  exports: [EmailService, TypeOrmModule],
})
export class EmailModule {}

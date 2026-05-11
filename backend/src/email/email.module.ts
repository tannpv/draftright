import { Module, Global } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { EmailService } from './email.service';
import { AppSettings } from '../admin/entities/app-settings.entity';

@Global()
@Module({
  imports: [TypeOrmModule.forFeature([AppSettings])],
  providers: [EmailService],
  exports: [EmailService],
})
export class EmailModule {}

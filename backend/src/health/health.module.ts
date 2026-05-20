import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { HealthController } from './health.controller';
import { AppSettings } from '../admin/entities/app-settings.entity';

@Module({
  imports: [TypeOrmModule.forFeature([AppSettings])],
  controllers: [HealthController],
})
export class HealthModule {}

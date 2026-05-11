import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ScheduleModule } from '@nestjs/schedule';
import { ErrorsController } from './errors.controller';
import { ErrorsService } from './errors.service';
import { ErrorReport } from './entities/error-report.entity';
import { FixProposalCron } from './fix-proposal.cron';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([ErrorReport]),
    ScheduleModule.forRoot(),
    AiProvidersModule,
  ],
  controllers: [ErrorsController],
  providers: [ErrorsService, FixProposalCron],
  exports: [ErrorsService, FixProposalCron],
})
export class ErrorsModule {}

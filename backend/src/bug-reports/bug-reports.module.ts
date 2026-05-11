import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BugReportsController } from './bug-reports.controller';
import { BugReportsService } from './bug-reports.service';
import { BugReport } from './entities/bug-report.entity';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { BugFixProposalCron } from './bug-fix-proposal.cron';

@Module({
  imports: [TypeOrmModule.forFeature([BugReport]), AiProvidersModule],
  controllers: [BugReportsController],
  providers: [BugReportsService, BugFixProposalCron],
  exports: [BugReportsService],
})
export class BugReportsModule {}

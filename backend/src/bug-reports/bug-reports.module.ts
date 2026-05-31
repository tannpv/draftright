import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BugReportsController } from './bug-reports.controller';
import { FeedbackController } from './feedback.controller';
import { BugReportsService } from './bug-reports.service';
import { BugReport } from './entities/bug-report.entity';
import { FeatureVote } from './entities/feature-vote.entity';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { BugFixProposalCron } from './bug-fix-proposal.cron';
import { JwtUserService } from './jwt-user.service';

@Module({
  imports: [TypeOrmModule.forFeature([BugReport, FeatureVote]), AiProvidersModule],
  controllers: [BugReportsController, FeedbackController],
  providers: [BugReportsService, BugFixProposalCron, JwtUserService],
  exports: [BugReportsService],
})
export class BugReportsModule {}

import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { RewriteController } from './rewrite.controller';
import { RewriteService } from './rewrite.service';
import { RewriteCacheService } from './rewrite-cache.service';
import { RewriteLogService } from './rewrite-log.service';
import { RewriteLog } from './entities/rewrite-log.entity';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { AuthModule } from '../auth/auth.module';
import { MetricsModule } from '../common/metrics/metrics.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([RewriteLog]),
    SubscriptionsModule,
    UsageModule,
    AiProvidersModule,
    AuthModule,
    MetricsModule,
  ],
  controllers: [RewriteController],
  providers: [RewriteService, RewriteCacheService, RewriteLogService],
  exports: [RewriteLogService],
})
export class RewriteModule {}

import { Module } from '@nestjs/common';
import { RewriteController } from './rewrite.controller';
import { RewriteService } from './rewrite.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';

@Module({
  imports: [SubscriptionsModule, UsageModule, AiProvidersModule],
  controllers: [RewriteController],
  providers: [RewriteService],
})
export class RewriteModule {}

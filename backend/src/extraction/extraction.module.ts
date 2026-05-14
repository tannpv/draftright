import { Module } from '@nestjs/common';
import { ExtractionController } from './extraction.controller';
import { ExtractionService } from './extraction.service';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { AuthModule } from '../auth/auth.module';

@Module({
  imports: [AiProvidersModule, AuthModule],
  controllers: [ExtractionController],
  providers: [ExtractionService],
})
export class ExtractionModule {}

import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { AiProvider } from './entities/ai-provider.entity';
import { AiProvidersService } from './ai-providers.service';

@Module({
  imports: [TypeOrmModule.forFeature([AiProvider])],
  providers: [AiProvidersService],
  exports: [AiProvidersService],
})
export class AiProvidersModule {}

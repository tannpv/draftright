import { Module } from '@nestjs/common';
import { ImePacksController } from './ime-packs.controller';
import { ImePacksService } from './ime-packs.service';

@Module({
  controllers: [ImePacksController],
  providers: [ImePacksService],
  exports: [ImePacksService],
})
export class ImePacksModule {}

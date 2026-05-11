import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { UpdatesController } from './updates.controller';
import { ReleasesService } from './releases.service';
import { AppRelease } from './entities/app-release.entity';

@Module({
  imports: [TypeOrmModule.forFeature([AppRelease])],
  controllers: [UpdatesController],
  providers: [ReleasesService],
  exports: [ReleasesService],
})
export class UpdatesModule {}

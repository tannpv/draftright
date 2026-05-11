import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { UpdatesController } from './updates.controller';
import { ReleasesService } from './releases.service';
import { PoliciesService } from './policies.service';
import { AppRelease } from './entities/app-release.entity';
import { AppReleasePolicy } from './entities/app-release-policy.entity';

@Module({
  imports: [TypeOrmModule.forFeature([AppRelease, AppReleasePolicy])],
  controllers: [UpdatesController],
  providers: [ReleasesService, PoliciesService],
  exports: [ReleasesService, PoliciesService],
})
export class UpdatesModule {}

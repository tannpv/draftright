import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ErrorsController } from './errors.controller';
import { ErrorsService } from './errors.service';
import { ErrorReport } from './entities/error-report.entity';

@Module({
  imports: [TypeOrmModule.forFeature([ErrorReport])],
  controllers: [ErrorsController],
  providers: [ErrorsService],
  exports: [ErrorsService],
})
export class ErrorsModule {}

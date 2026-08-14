import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { UserContext } from './entities/user-context.entity';
import { UserContextService } from './user-context.service';
import { UserContextController } from './user-context.controller';

@Module({
  imports: [TypeOrmModule.forFeature([UserContext])],
  controllers: [UserContextController],
  providers: [UserContextService],
  exports: [UserContextService],
})
export class UserContextModule {}

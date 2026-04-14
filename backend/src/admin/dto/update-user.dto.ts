import { IsOptional, IsBoolean, IsString, IsIn } from 'class-validator';
import { ApiPropertyOptional } from '@nestjs/swagger';

export class UpdateUserDto {
  @ApiPropertyOptional()
  @IsOptional()
  @IsBoolean()
  is_active?: boolean;

  @ApiPropertyOptional({ enum: ['user'] })
  @IsOptional()
  @IsIn(['user'])
  role?: string;

  @ApiPropertyOptional()
  @IsOptional()
  @IsString()
  name?: string;
}

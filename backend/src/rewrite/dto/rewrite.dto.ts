import { IsString, IsOptional, IsIn } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

export class RewriteDto {
  @ApiProperty({ example: 'This is some text to rewrite' })
  @IsString()
  text: string;

  @ApiProperty({ example: 'polished', enum: ['simple', 'natural', 'polished', 'concise', 'technical', 'translate'] })
  @IsIn(['simple', 'natural', 'polished', 'concise', 'technical', 'translate'])
  tone: string;

  @ApiPropertyOptional({ example: 'Vietnamese' })
  @IsOptional()
  @IsString()
  target_language?: string;
}

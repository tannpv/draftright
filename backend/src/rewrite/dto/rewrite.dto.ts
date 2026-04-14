import { IsString, IsOptional, IsIn } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

export class RewriteDto {
  @ApiProperty({ example: 'This is some text to rewrite' })
  @IsString()
  text: string;

  @ApiProperty({ example: 'polished', enum: ['simple', 'natural', 'polished', 'concise', 'technical', 'claude', 'grammar_check', 'translate'] })
  @IsIn(['simple', 'natural', 'polished', 'concise', 'technical', 'claude', 'grammar_check', 'translate'])
  tone: string;

  @ApiPropertyOptional({ example: 'Vietnamese' })
  @IsOptional()
  @IsString()
  target_language?: string;

  @ApiPropertyOptional({ example: 'English', description: 'Source language hint (auto-detected if not provided)' })
  @IsOptional()
  @IsString()
  source_language?: string;
}

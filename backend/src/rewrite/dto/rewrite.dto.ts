import { IsString, IsOptional, IsIn } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { TONE_IDS, INPUT_KIND_IDS } from '../tones';

export class RewriteDto {
  @ApiProperty({ example: 'This is some text to rewrite' })
  @IsString()
  text: string;

  @ApiProperty({ example: 'polished', enum: TONE_IDS })
  @IsIn(TONE_IDS)
  tone: string;

  @ApiPropertyOptional({ example: 'Vietnamese' })
  @IsOptional()
  @IsString()
  target_language?: string;

  @ApiPropertyOptional({ example: 'English', description: 'Source language hint (auto-detected if not provided)' })
  @IsOptional()
  @IsString()
  source_language?: string;

  @ApiPropertyOptional({ example: 'speech', enum: INPUT_KIND_IDS, description: 'Marks dictated input; speech adds a cleanup preamble to the prompt' })
  @IsOptional()
  @IsIn(INPUT_KIND_IDS as unknown as string[])
  input_kind?: string;
}

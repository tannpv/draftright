import { IsString, IsIn, IsOptional } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

export class CreateCheckoutDto {
  @ApiProperty({ example: 'plan-uuid-here' })
  @IsString()
  plan_id: string;

  @ApiProperty({ example: 'vietqr', enum: ['stripe', 'paypal', 'vietqr', 'bank_transfer'] })
  @IsIn(['stripe', 'paypal', 'momo', 'vietqr', 'bank_transfer'])
  method: string;

  @ApiPropertyOptional({ example: 'https://draftright.app/payment/success' })
  @IsOptional()
  @IsString()
  success_url?: string;

  @ApiPropertyOptional({ example: 'https://draftright.app/payment/cancel' })
  @IsOptional()
  @IsString()
  cancel_url?: string;
}

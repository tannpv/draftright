import { IsString, IsIn, IsOptional } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { PaymentMethod } from '../entities/payment.entity';

/**
 * Source the allowed `method` values from the PaymentMethod enum so
 * adding a new method doesn't require touching the DTO too — the
 * enum is the single source of truth for valid wire names.
 */
const PAYMENT_METHODS = Object.values(PaymentMethod) as string[];

export class CreateCheckoutDto {
  @ApiProperty({ example: 'plan-uuid-here' })
  @IsString()
  plan_id: string;

  @ApiProperty({ example: 'vietqr', enum: PAYMENT_METHODS })
  @IsIn(PAYMENT_METHODS)
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

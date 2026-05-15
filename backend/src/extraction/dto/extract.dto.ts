import {
  IsArray,
  IsEnum,
  IsNumber,
  IsObject,
  IsOptional,
  IsString,
  MaxLength,
  ArrayMaxSize,
} from 'class-validator';

export enum EntityKind {
  Phone = 'phone',
  Email = 'email',
  Url = 'url',
  Otp = 'otp',
  CreditCard = 'creditCard',
  Address = 'address',
  PersonName = 'personName',
  DateTime = 'dateTime',
  BankAccount = 'bankAccount',
}

export class ExtractRequestDto {
  @IsString()
  @MaxLength(8000)
  text!: string;

  @IsOptional()
  @IsArray()
  @ArrayMaxSize(20)
  @IsEnum(EntityKind, { each: true })
  kinds?: EntityKind[];
}

export interface ExtractedEntityDto {
  kind: EntityKind;
  value: string;
  display: string;
  start: number;
  end: number;
  confidence: number;
  meta?: Record<string, string>;
}

export interface ExtractResponseDto {
  entities: ExtractedEntityDto[];
  provider: string;
  tokensUsed: number;
}

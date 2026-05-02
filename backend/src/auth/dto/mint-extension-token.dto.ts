import { IsString, IsUUID, Length, Matches } from 'class-validator';

export class MintExtensionTokenDto {
  @IsUUID('4')
  device_id: string;

  @IsString()
  @Length(1, 64)
  @Matches(/^[A-Za-z0-9 _.\-]+$/, {
    message: 'device_name must be alphanumeric/space/_/./- only',
  })
  device_name: string;
}

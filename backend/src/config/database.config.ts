import { TypeOrmModuleOptions } from '@nestjs/typeorm';

export const databaseConfig = (): TypeOrmModuleOptions => ({
  type: 'postgres',
  url: process.env.DATABASE_URL || 'postgresql://draftright:password@localhost:5432/draftright',
  autoLoadEntities: true,
  synchronize: process.env.NODE_ENV !== 'production',
  logging: process.env.NODE_ENV === 'development',
});

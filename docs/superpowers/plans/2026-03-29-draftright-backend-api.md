# DraftRight Backend API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a NestJS backend API with PostgreSQL that proxies AI rewrite requests, manages user auth, subscriptions with daily limits, and admin CRUD — deployed via Docker Compose.

**Architecture:** NestJS modular backend with TypeORM entities, JWT auth (access + refresh tokens), and a rewrite proxy that checks usage quotas before forwarding to configured AI providers. Admin endpoints manage users, plans, and providers.

**Tech Stack:** NestJS 10+, TypeScript, TypeORM, PostgreSQL 16, JWT, bcrypt, class-validator, Swagger, Docker Compose

**Spec:** `docs/superpowers/specs/2026-03-29-draftright-backend-api-design.md`

---

### Task 1: Project Scaffold + Docker Compose

**Files:**
- Create: `backend/package.json`
- Create: `backend/tsconfig.json`
- Create: `backend/tsconfig.build.json`
- Create: `backend/nest-cli.json`
- Create: `backend/src/main.ts`
- Create: `backend/src/app.module.ts`
- Create: `backend/src/config/database.config.ts`
- Create: `backend/Dockerfile`
- Create: `backend/.env.example`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create NestJS project**

```bash
cd /opt/openAi/DraftRight
mkdir -p backend/src/config
```

Create `backend/package.json`:

```json
{
  "name": "draftright-backend",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "build": "nest build",
    "start": "nest start",
    "start:dev": "nest start --watch",
    "start:prod": "node dist/main",
    "test": "jest",
    "test:e2e": "jest --config ./test/jest-e2e.json",
    "seed": "ts-node src/seed.ts"
  },
  "dependencies": {
    "@nestjs/common": "^10.0.0",
    "@nestjs/core": "^10.0.0",
    "@nestjs/jwt": "^10.0.0",
    "@nestjs/passport": "^10.0.0",
    "@nestjs/platform-express": "^10.0.0",
    "@nestjs/swagger": "^7.0.0",
    "@nestjs/typeorm": "^10.0.0",
    "bcrypt": "^5.1.0",
    "class-transformer": "^0.5.1",
    "class-validator": "^0.14.0",
    "passport": "^0.7.0",
    "passport-jwt": "^4.0.1",
    "pg": "^8.11.0",
    "reflect-metadata": "^0.2.0",
    "rxjs": "^7.8.0",
    "typeorm": "^0.3.17"
  },
  "devDependencies": {
    "@nestjs/cli": "^10.0.0",
    "@nestjs/testing": "^10.0.0",
    "@types/bcrypt": "^5.0.0",
    "@types/jest": "^29.5.0",
    "@types/node": "^20.0.0",
    "@types/passport-jwt": "^4.0.0",
    "jest": "^29.5.0",
    "ts-jest": "^29.1.0",
    "ts-node": "^10.9.0",
    "typescript": "^5.1.0"
  },
  "jest": {
    "moduleFileExtensions": ["js", "json", "ts"],
    "rootDir": "src",
    "testRegex": ".*\\.spec\\.ts$",
    "transform": { "^.+\\.(t|j)s$": "ts-jest" },
    "collectCoverageFrom": ["**/*.(t|j)s"],
    "coverageDirectory": "../coverage",
    "testEnvironment": "node"
  }
}
```

- [ ] **Step 2: Create TypeScript config**

Create `backend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "declaration": true,
    "removeComments": true,
    "emitDecoratorMetadata": true,
    "experimentalDecorators": true,
    "allowSyntheticDefaultImports": true,
    "target": "ES2021",
    "sourceMap": true,
    "outDir": "./dist",
    "baseUrl": "./",
    "incremental": true,
    "skipLibCheck": true,
    "strictNullChecks": true,
    "noImplicitAny": true,
    "strictBindCallApply": true,
    "forceConsistentCasingInFileNames": true,
    "noFallthroughCasesInSwitch": true
  }
}
```

Create `backend/tsconfig.build.json`:

```json
{
  "extends": "./tsconfig.json",
  "exclude": ["node_modules", "test", "dist", "**/*spec.ts"]
}
```

Create `backend/nest-cli.json`:

```json
{
  "$schema": "https://json.schemastore.org/nest-cli",
  "collection": "@nestjs/schematics",
  "sourceRoot": "src",
  "compilerOptions": {
    "deleteOutDir": true
  }
}
```

- [ ] **Step 3: Create database config**

Create `backend/src/config/database.config.ts`:

```typescript
import { TypeOrmModuleOptions } from '@nestjs/typeorm';

export const databaseConfig = (): TypeOrmModuleOptions => ({
  type: 'postgres',
  url: process.env.DATABASE_URL || 'postgresql://draftright:password@localhost:5432/draftright',
  autoLoadEntities: true,
  synchronize: process.env.NODE_ENV !== 'production',
  logging: process.env.NODE_ENV === 'development',
});
```

- [ ] **Step 4: Create app.module.ts**

Create `backend/src/app.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
  ],
})
export class AppModule {}
```

- [ ] **Step 5: Create main.ts**

Create `backend/src/main.ts`:

```typescript
import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  app.useGlobalPipes(new ValidationPipe({
    whitelist: true,
    forbidNonWhitelisted: true,
    transform: true,
  }));

  app.enableCors();

  const config = new DocumentBuilder()
    .setTitle('DraftRight API')
    .setDescription('AI-powered text rewriting backend')
    .setVersion('1.0')
    .addBearerAuth()
    .build();
  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api/docs', app, document);

  const port = process.env.PORT || 3000;
  await app.listen(port);
  console.log(`DraftRight API running on port ${port}`);
}
bootstrap();
```

- [ ] **Step 6: Create Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
EXPOSE 3000
CMD ["node", "dist/main"]
```

- [ ] **Step 7: Create docker-compose.yml**

Create `docker-compose.yml` (project root):

```yaml
services:
  backend:
    build: ./backend
    ports:
      - "3000:3000"
    environment:
      - DATABASE_URL=postgresql://draftright:password@postgres:5432/draftright
      - JWT_SECRET=${JWT_SECRET}
      - JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ADMIN_EMAIL=${ADMIN_EMAIL}
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=draftright
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=draftright
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-ONLY", "pg_isready", "-U", "draftright"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
```

- [ ] **Step 8: Create .env.example**

Create `backend/.env.example`:

```
DATABASE_URL=postgresql://draftright:password@localhost:5432/draftright
JWT_SECRET=change-me-to-random-string
JWT_REFRESH_SECRET=change-me-to-another-random-string
OPENAI_API_KEY=sk-your-openai-key
ADMIN_EMAIL=admin@draftright.com
ADMIN_PASSWORD=change-me
PORT=3000
```

- [ ] **Step 9: Install dependencies and verify build**

```bash
cd /opt/openAi/DraftRight/backend && npm install && npm run build
```

Expected: compiles with no errors.

- [ ] **Step 10: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend/ docker-compose.yml
git commit -m "feat: scaffold NestJS backend with Docker Compose"
```

---

### Task 2: User Entity + Users Module

**Files:**
- Create: `backend/src/users/entities/user.entity.ts`
- Create: `backend/src/users/users.service.ts`
- Create: `backend/src/users/users.module.ts`

- [ ] **Step 1: Create User entity**

Create `backend/src/users/entities/user.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum UserRole {
  USER = 'user',
  ADMIN = 'admin',
}

@Entity('users')
export class User {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 255, unique: true })
  email: string;

  @Column({ type: 'varchar', length: 255 })
  password_hash: string;

  @Column({ type: 'varchar', length: 255 })
  name: string;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @Column({ type: 'enum', enum: UserRole, default: UserRole.USER })
  role: UserRole;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
```

- [ ] **Step 2: Create UsersService**

Create `backend/src/users/users.service.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { User } from './entities/user.entity';

@Injectable()
export class UsersService {
  constructor(
    @InjectRepository(User)
    private readonly usersRepo: Repository<User>,
  ) {}

  async findByEmail(email: string): Promise<User | null> {
    return this.usersRepo.findOne({ where: { email } });
  }

  async findById(id: string): Promise<User | null> {
    return this.usersRepo.findOne({ where: { id } });
  }

  async create(data: { email: string; password_hash: string; name: string; role?: string }): Promise<User> {
    const user = this.usersRepo.create(data);
    return this.usersRepo.save(user);
  }

  async update(id: string, data: Partial<User>): Promise<User> {
    await this.usersRepo.update(id, data);
    return this.usersRepo.findOneOrFail({ where: { id } });
  }

  async count(): Promise<number> {
    return this.usersRepo.count();
  }

  async findAll(options: { search?: string; page?: number; limit?: number }): Promise<{ users: User[]; total: number }> {
    const { search, page = 1, limit = 20 } = options;
    const qb = this.usersRepo.createQueryBuilder('user');

    if (search) {
      qb.where('user.email ILIKE :search OR user.name ILIKE :search', { search: `%${search}%` });
    }

    qb.orderBy('user.created_at', 'DESC')
      .skip((page - 1) * limit)
      .take(limit);

    const [users, total] = await qb.getManyAndCount();
    return { users, total };
  }
}
```

- [ ] **Step 3: Create UsersModule**

Create `backend/src/users/users.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { User } from './entities/user.entity';
import { UsersService } from './users.service';

@Module({
  imports: [TypeOrmModule.forFeature([User])],
  providers: [UsersService],
  exports: [UsersService],
})
export class UsersModule {}
```

- [ ] **Step 4: Register in AppModule**

Update `backend/src/app.module.ts` — add `UsersModule` to imports:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 5: Verify build**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend/src/users/ backend/src/app.module.ts
git commit -m "feat: add User entity and UsersService"
```

---

### Task 3: Plans Module

**Files:**
- Create: `backend/src/plans/entities/plan.entity.ts`
- Create: `backend/src/plans/plans.service.ts`
- Create: `backend/src/plans/plans.module.ts`

- [ ] **Step 1: Create Plan entity**

Create `backend/src/plans/entities/plan.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum BillingPeriod {
  NONE = 'none',
  MONTHLY = 'monthly',
  YEARLY = 'yearly',
}

@Entity('plans')
export class Plan {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 100 })
  name: string;

  @Column({ type: 'int' })
  daily_limit: number; // -1 = unlimited

  @Column({ type: 'int', default: 0 })
  price_cents: number;

  @Column({ type: 'enum', enum: BillingPeriod, default: BillingPeriod.NONE })
  billing_period: BillingPeriod;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
```

- [ ] **Step 2: Create PlansService**

Create `backend/src/plans/plans.service.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Plan } from './entities/plan.entity';

@Injectable()
export class PlansService {
  constructor(
    @InjectRepository(Plan)
    private readonly plansRepo: Repository<Plan>,
  ) {}

  async findAll(): Promise<Plan[]> {
    return this.plansRepo.find({ order: { created_at: 'ASC' } });
  }

  async findById(id: string): Promise<Plan | null> {
    return this.plansRepo.findOne({ where: { id } });
  }

  async findFreePlan(): Promise<Plan> {
    const plan = await this.plansRepo.findOne({ where: { billing_period: 'none' as any, is_active: true } });
    if (!plan) throw new Error('Free plan not found. Run seed first.');
    return plan;
  }

  async create(data: Partial<Plan>): Promise<Plan> {
    const plan = this.plansRepo.create(data);
    return this.plansRepo.save(plan);
  }

  async update(id: string, data: Partial<Plan>): Promise<Plan> {
    await this.plansRepo.update(id, data);
    return this.plansRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.plansRepo.update(id, { is_active: false });
  }
}
```

- [ ] **Step 3: Create PlansModule**

Create `backend/src/plans/plans.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Plan } from './entities/plan.entity';
import { PlansService } from './plans.service';

@Module({
  imports: [TypeOrmModule.forFeature([Plan])],
  providers: [PlansService],
  exports: [PlansService],
})
export class PlansModule {}
```

- [ ] **Step 4: Register in AppModule**

Update `backend/src/app.module.ts` — add `PlansModule`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
    PlansModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 5: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/plans/ backend/src/app.module.ts
git commit -m "feat: add Plan entity and PlansService"
```

---

### Task 4: Subscriptions Module

**Files:**
- Create: `backend/src/subscriptions/entities/subscription.entity.ts`
- Create: `backend/src/subscriptions/subscriptions.service.ts`
- Create: `backend/src/subscriptions/subscriptions.controller.ts`
- Create: `backend/src/subscriptions/subscriptions.module.ts`

- [ ] **Step 1: Create Subscription entity**

Create `backend/src/subscriptions/entities/subscription.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
  ManyToOne, JoinColumn,
} from 'typeorm';
import { User } from '../../users/entities/user.entity';
import { Plan } from '../../plans/entities/plan.entity';

export enum SubscriptionStatus {
  ACTIVE = 'active',
  CANCELLED = 'cancelled',
  EXPIRED = 'expired',
}

export enum StoreType {
  GOOGLE_PLAY = 'google_play',
  APPLE_IAP = 'apple_iap',
  ADMIN_GRANTED = 'admin_granted',
}

@Entity('subscriptions')
export class Subscription {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  @ManyToOne(() => User)
  @JoinColumn({ name: 'user_id' })
  user: User;

  @Column({ type: 'uuid' })
  plan_id: string;

  @ManyToOne(() => Plan)
  @JoinColumn({ name: 'plan_id' })
  plan: Plan;

  @Column({ type: 'enum', enum: SubscriptionStatus })
  status: SubscriptionStatus;

  @Column({ type: 'enum', enum: StoreType })
  store_type: StoreType;

  @Column({ type: 'varchar', length: 500, nullable: true })
  store_transaction_id: string | null;

  @Column({ type: 'timestamp' })
  started_at: Date;

  @Column({ type: 'timestamp', nullable: true })
  expires_at: Date | null;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
```

- [ ] **Step 2: Create SubscriptionsService**

Create `backend/src/subscriptions/subscriptions.service.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Subscription, SubscriptionStatus, StoreType } from './entities/subscription.entity';

@Injectable()
export class SubscriptionsService {
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
  ) {}

  async findActiveByUserId(userId: string): Promise<Subscription | null> {
    return this.subsRepo.findOne({
      where: { user_id: userId, status: SubscriptionStatus.ACTIVE },
      relations: ['plan'],
      order: { created_at: 'DESC' },
    });
  }

  async createFreeSubscription(userId: string, planId: string): Promise<Subscription> {
    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: StoreType.ADMIN_GRANTED,
      started_at: new Date(),
      expires_at: null,
    });
    return this.subsRepo.save(sub);
  }

  async grant(userId: string, planId: string, expiresAt?: Date): Promise<Subscription> {
    // Expire any existing active subscription
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );

    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: StoreType.ADMIN_GRANTED,
      started_at: new Date(),
      expires_at: expiresAt || null,
    });
    return this.subsRepo.save(sub);
  }

  async verifyAndActivate(
    userId: string,
    planId: string,
    storeType: StoreType,
    transactionId: string,
    expiresAt: Date,
  ): Promise<Subscription> {
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );

    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: storeType,
      store_transaction_id: transactionId,
      started_at: new Date(),
      expires_at: expiresAt,
    });
    return this.subsRepo.save(sub);
  }

  async countActive(): Promise<number> {
    return this.subsRepo.count({ where: { status: SubscriptionStatus.ACTIVE } });
  }
}
```

- [ ] **Step 3: Create SubscriptionsController**

Create `backend/src/subscriptions/subscriptions.controller.ts`:

```typescript
import { Controller, Get, Post, Body, UseGuards, Request } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { SubscriptionsService } from './subscriptions.service';
import { UsageService } from '../usage/usage.service';

@ApiTags('subscription')
@Controller('subscription')
export class SubscriptionsController {
  constructor(
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
  ) {}

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Get()
  async getMySubscription(@Request() req: any) {
    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    const usageToday = await this.usageService.countTodayByUser(req.user.id);

    return {
      plan: sub?.plan ? {
        name: sub.plan.name,
        daily_limit: sub.plan.daily_limit,
        billing_period: sub.plan.billing_period,
      } : null,
      status: sub?.status || null,
      expires_at: sub?.expires_at || null,
      usage_today: usageToday,
    };
  }

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Post('verify-receipt')
  async verifyReceipt(@Request() req: any, @Body() body: { store_type: string; receipt_data: string; product_id: string }) {
    // Receipt validation with Apple/Google is a future integration.
    // For now, return the current subscription.
    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    return {
      subscription: sub ? {
        plan: sub.plan?.name,
        status: sub.status,
        expires_at: sub.expires_at,
      } : null,
    };
  }
}
```

- [ ] **Step 4: Create SubscriptionsModule**

Create `backend/src/subscriptions/subscriptions.module.ts`:

```typescript
import { Module, forwardRef } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Subscription } from './entities/subscription.entity';
import { SubscriptionsService } from './subscriptions.service';
import { SubscriptionsController } from './subscriptions.controller';
import { UsageModule } from '../usage/usage.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([Subscription]),
    forwardRef(() => UsageModule),
  ],
  controllers: [SubscriptionsController],
  providers: [SubscriptionsService],
  exports: [SubscriptionsService],
})
export class SubscriptionsModule {}
```

Note: SubscriptionsController depends on UsageService (built in Task 7) and JwtAuthGuard (built in Task 6). This module will compile but the controller won't function until those are built. We register it in AppModule after Task 6.

- [ ] **Step 5: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/subscriptions/
git commit -m "feat: add Subscription entity, service, and controller"
```

---

### Task 5: AI Providers Module

**Files:**
- Create: `backend/src/ai-providers/entities/ai-provider.entity.ts`
- Create: `backend/src/ai-providers/ai-providers.service.ts`
- Create: `backend/src/ai-providers/ai-providers.module.ts`

- [ ] **Step 1: Create AiProvider entity**

Create `backend/src/ai-providers/entities/ai-provider.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum AiProviderType {
  OPENAI = 'openai',
  OLLAMA = 'ollama',
  CUSTOM = 'custom',
}

@Entity('ai_providers')
export class AiProvider {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 255 })
  name: string;

  @Column({ type: 'enum', enum: AiProviderType })
  type: AiProviderType;

  @Column({ type: 'varchar', length: 500 })
  endpoint_url: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  api_key: string;

  @Column({ type: 'varchar', length: 100 })
  model: string;

  @Column({ type: 'decimal', precision: 3, scale: 2, default: 0.3 })
  temperature: number;

  @Column({ type: 'boolean', default: false })
  is_default: boolean;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
```

- [ ] **Step 2: Create AiProvidersService**

Create `backend/src/ai-providers/ai-providers.service.ts`:

```typescript
import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AiProvider } from './entities/ai-provider.entity';

@Injectable()
export class AiProvidersService {
  constructor(
    @InjectRepository(AiProvider)
    private readonly providersRepo: Repository<AiProvider>,
  ) {}

  async findAll(): Promise<AiProvider[]> {
    return this.providersRepo.find({ order: { created_at: 'ASC' } });
  }

  async findById(id: string): Promise<AiProvider | null> {
    return this.providersRepo.findOne({ where: { id } });
  }

  async findDefault(): Promise<AiProvider> {
    const provider = await this.providersRepo.findOne({ where: { is_default: true, is_active: true } });
    if (!provider) throw new BadRequestException('No default AI provider configured');
    return provider;
  }

  async create(data: Partial<AiProvider>): Promise<AiProvider> {
    const provider = this.providersRepo.create(data);
    return this.providersRepo.save(provider);
  }

  async update(id: string, data: Partial<AiProvider>): Promise<AiProvider> {
    if (data.is_default) {
      // Unset other defaults
      await this.providersRepo.update({}, { is_default: false });
    }
    await this.providersRepo.update(id, data);
    return this.providersRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.providersRepo.update(id, { is_active: false, is_default: false });
  }

  async callProvider(provider: AiProvider, systemPrompt: string, userText: string): Promise<{ text: string; responseTimeMs: number }> {
    const startTime = Date.now();

    const body = {
      model: provider.model,
      temperature: Number(provider.temperature),
      messages: [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: userText },
      ],
    };

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (provider.api_key) {
      headers['Authorization'] = `Bearer ${provider.api_key}`;
    }

    const response = await fetch(provider.endpoint_url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`AI provider error (${response.status}): ${errorText}`);
    }

    const json = await response.json();
    const text = json.choices?.[0]?.message?.content?.trim() || '';
    const responseTimeMs = Date.now() - startTime;

    return { text, responseTimeMs };
  }
}
```

- [ ] **Step 3: Create AiProvidersModule**

Create `backend/src/ai-providers/ai-providers.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { AiProvider } from './entities/ai-provider.entity';
import { AiProvidersService } from './ai-providers.service';

@Module({
  imports: [TypeOrmModule.forFeature([AiProvider])],
  providers: [AiProvidersService],
  exports: [AiProvidersService],
})
export class AiProvidersModule {}
```

- [ ] **Step 4: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/ai-providers/
git commit -m "feat: add AiProvider entity, service with proxy call"
```

---

### Task 6: Auth Module (JWT, Register, Login)

**Files:**
- Create: `backend/src/auth/dto/register.dto.ts`
- Create: `backend/src/auth/dto/login.dto.ts`
- Create: `backend/src/auth/jwt.strategy.ts`
- Create: `backend/src/auth/jwt-auth.guard.ts`
- Create: `backend/src/auth/auth.service.ts`
- Create: `backend/src/auth/auth.controller.ts`
- Create: `backend/src/auth/auth.module.ts`
- Create: `backend/src/common/decorators/roles.decorator.ts`
- Create: `backend/src/common/guards/roles.guard.ts`

- [ ] **Step 1: Create DTOs**

Create `backend/src/auth/dto/register.dto.ts`:

```typescript
import { IsEmail, IsString, MinLength } from 'class-validator';
import { ApiProperty } from '@nestjs/swagger';

export class RegisterDto {
  @ApiProperty({ example: 'user@example.com' })
  @IsEmail()
  email: string;

  @ApiProperty({ example: 'password123' })
  @IsString()
  @MinLength(8)
  password: string;

  @ApiProperty({ example: 'John Doe' })
  @IsString()
  @MinLength(1)
  name: string;
}
```

Create `backend/src/auth/dto/login.dto.ts`:

```typescript
import { IsEmail, IsString } from 'class-validator';
import { ApiProperty } from '@nestjs/swagger';

export class LoginDto {
  @ApiProperty({ example: 'user@example.com' })
  @IsEmail()
  email: string;

  @ApiProperty({ example: 'password123' })
  @IsString()
  password: string;
}
```

- [ ] **Step 2: Create JWT strategy and guard**

Create `backend/src/auth/jwt.strategy.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { PassportStrategy } from '@nestjs/passport';
import { ExtractJwt, Strategy } from 'passport-jwt';

@Injectable()
export class JwtStrategy extends PassportStrategy(Strategy) {
  constructor() {
    super({
      jwtFromRequest: ExtractJwt.fromAuthHeaderAsBearerToken(),
      ignoreExpiration: false,
      secretOrKey: process.env.JWT_SECRET || 'dev-secret',
    });
  }

  async validate(payload: { sub: string; email: string; role: string }) {
    return { id: payload.sub, email: payload.email, role: payload.role };
  }
}
```

Create `backend/src/auth/jwt-auth.guard.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';

@Injectable()
export class JwtAuthGuard extends AuthGuard('jwt') {}
```

- [ ] **Step 3: Create roles decorator and guard**

Create `backend/src/common/decorators/roles.decorator.ts`:

```typescript
import { SetMetadata } from '@nestjs/common';

export const Roles = (...roles: string[]) => SetMetadata('roles', roles);
```

Create `backend/src/common/guards/roles.guard.ts`:

```typescript
import { Injectable, CanActivate, ExecutionContext, ForbiddenException } from '@nestjs/common';
import { Reflector } from '@nestjs/core';

@Injectable()
export class RolesGuard implements CanActivate {
  constructor(private reflector: Reflector) {}

  canActivate(context: ExecutionContext): boolean {
    const requiredRoles = this.reflector.getAllAndOverride<string[]>('roles', [
      context.getHandler(),
      context.getClass(),
    ]);
    if (!requiredRoles) return true;

    const { user } = context.switchToHttp().getRequest();
    if (!requiredRoles.includes(user.role)) {
      throw new ForbiddenException('Admin access required');
    }
    return true;
  }
}
```

- [ ] **Step 4: Create AuthService**

Create `backend/src/auth/auth.service.ts`:

```typescript
import { Injectable, UnauthorizedException, ConflictException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import * as bcrypt from 'bcrypt';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';

@Injectable()
export class AuthService {
  constructor(
    private readonly usersService: UsersService,
    private readonly jwtService: JwtService,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
  ) {}

  private generateTokens(user: { id: string; email: string; role: string }) {
    const payload = { sub: user.id, email: user.email, role: user.role };

    const access_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_SECRET || 'dev-secret',
      expiresIn: '15m',
    });

    const refresh_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      expiresIn: '7d',
    });

    return { access_token, refresh_token };
  }

  async register(email: string, password: string, name: string) {
    const existing = await this.usersService.findByEmail(email);
    if (existing) throw new ConflictException('Email already registered');

    const password_hash = await bcrypt.hash(password, 10);
    const user = await this.usersService.create({ email, password_hash, name });

    // Assign free plan
    const freePlan = await this.plansService.findFreePlan();
    await this.subscriptionsService.createFreeSubscription(user.id, freePlan.id);

    const tokens = this.generateTokens(user);
    return {
      ...tokens,
      user: { id: user.id, email: user.email, name: user.name },
    };
  }

  async login(email: string, password: string) {
    const user = await this.usersService.findByEmail(email);
    if (!user) throw new UnauthorizedException('Invalid credentials');

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) throw new UnauthorizedException('Invalid credentials');

    if (!user.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = this.generateTokens(user);
    return {
      ...tokens,
      user: { id: user.id, email: user.email, name: user.name },
    };
  }

  async refresh(refreshToken: string) {
    try {
      const payload = this.jwtService.verify(refreshToken, {
        secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      });
      const user = await this.usersService.findById(payload.sub);
      if (!user || !user.is_active) throw new UnauthorizedException();
      return this.generateTokens(user);
    } catch {
      throw new UnauthorizedException('Invalid refresh token');
    }
  }

  async changePassword(userId: string, currentPassword: string, newPassword: string) {
    const user = await this.usersService.findById(userId);
    if (!user) throw new UnauthorizedException();

    const valid = await bcrypt.compare(currentPassword, user.password_hash);
    if (!valid) throw new UnauthorizedException('Current password is incorrect');

    const password_hash = await bcrypt.hash(newPassword, 10);
    await this.usersService.update(userId, { password_hash });
    return { success: true };
  }
}
```

- [ ] **Step 5: Create AuthController**

Create `backend/src/auth/auth.controller.ts`:

```typescript
import { Controller, Post, Body, UseGuards, Request } from '@nestjs/common';
import { ApiTags } from '@nestjs/swagger';
import { AuthService } from './auth.service';
import { RegisterDto } from './dto/register.dto';
import { LoginDto } from './dto/login.dto';
import { JwtAuthGuard } from './jwt-auth.guard';

@ApiTags('auth')
@Controller('auth')
export class AuthController {
  constructor(private readonly authService: AuthService) {}

  @Post('register')
  async register(@Body() dto: RegisterDto) {
    return this.authService.register(dto.email, dto.password, dto.name);
  }

  @Post('login')
  async login(@Body() dto: LoginDto) {
    return this.authService.login(dto.email, dto.password);
  }

  @Post('refresh')
  async refresh(@Body() body: { refresh_token: string }) {
    return this.authService.refresh(body.refresh_token);
  }

  @UseGuards(JwtAuthGuard)
  @Post('change-password')
  async changePassword(@Request() req: any, @Body() body: { current_password: string; new_password: string }) {
    return this.authService.changePassword(req.user.id, body.current_password, body.new_password);
  }
}
```

- [ ] **Step 6: Create AuthModule**

Create `backend/src/auth/auth.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { JwtModule } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import { AuthService } from './auth.service';
import { AuthController } from './auth.controller';
import { JwtStrategy } from './jwt.strategy';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [
    PassportModule,
    JwtModule.register({}),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
  ],
  controllers: [AuthController],
  providers: [AuthService, JwtStrategy],
  exports: [AuthService, JwtStrategy],
})
export class AuthModule {}
```

- [ ] **Step 7: Update AppModule with Auth + all modules so far**

Update `backend/src/app.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { AiProvidersModule } from './ai-providers/ai-providers.module';
import { AuthModule } from './auth/auth.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    AiProvidersModule,
    AuthModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 8: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/auth/ backend/src/common/ backend/src/app.module.ts
git commit -m "feat: add Auth module with JWT register, login, refresh, change-password"
```

---

### Task 7: Usage Module

**Files:**
- Create: `backend/src/usage/entities/usage-log.entity.ts`
- Create: `backend/src/usage/usage.service.ts`
- Create: `backend/src/usage/usage.module.ts`

- [ ] **Step 1: Create UsageLog entity**

Create `backend/src/usage/entities/usage-log.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, ManyToOne, JoinColumn, Index,
} from 'typeorm';
import { User } from '../../users/entities/user.entity';
import { AiProvider } from '../../ai-providers/entities/ai-provider.entity';

@Entity('usage_logs')
@Index(['user_id', 'created_at'])
export class UsageLog {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  @ManyToOne(() => User)
  @JoinColumn({ name: 'user_id' })
  user: User;

  @Column({ type: 'varchar', length: 20 })
  tone: string;

  @Column({ type: 'int' })
  input_length: number;

  @Column({ type: 'int' })
  output_length: number;

  @Column({ type: 'uuid' })
  ai_provider_id: string;

  @ManyToOne(() => AiProvider)
  @JoinColumn({ name: 'ai_provider_id' })
  ai_provider: AiProvider;

  @Column({ type: 'int' })
  response_time_ms: number;

  @CreateDateColumn()
  created_at: Date;
}
```

- [ ] **Step 2: Create UsageService**

Create `backend/src/usage/usage.service.ts`:

```typescript
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual } from 'typeorm';
import { UsageLog } from './entities/usage-log.entity';

@Injectable()
export class UsageService {
  constructor(
    @InjectRepository(UsageLog)
    private readonly usageRepo: Repository<UsageLog>,
  ) {}

  async countTodayByUser(userId: string): Promise<number> {
    const todayStart = new Date();
    todayStart.setHours(0, 0, 0, 0);

    return this.usageRepo.count({
      where: {
        user_id: userId,
        created_at: MoreThanOrEqual(todayStart),
      },
    });
  }

  async log(data: {
    user_id: string;
    tone: string;
    input_length: number;
    output_length: number;
    ai_provider_id: string;
    response_time_ms: number;
  }): Promise<UsageLog> {
    const entry = this.usageRepo.create(data);
    return this.usageRepo.save(entry);
  }

  async countToday(): Promise<number> {
    const todayStart = new Date();
    todayStart.setHours(0, 0, 0, 0);
    return this.usageRepo.count({ where: { created_at: MoreThanOrEqual(todayStart) } });
  }

  async countThisMonth(): Promise<number> {
    const monthStart = new Date();
    monthStart.setDate(1);
    monthStart.setHours(0, 0, 0, 0);
    return this.usageRepo.count({ where: { created_at: MoreThanOrEqual(monthStart) } });
  }

  async findRecentByUser(userId: string, limit: number = 20): Promise<UsageLog[]> {
    return this.usageRepo.find({
      where: { user_id: userId },
      order: { created_at: 'DESC' },
      take: limit,
    });
  }
}
```

- [ ] **Step 3: Create UsageModule**

Create `backend/src/usage/usage.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { UsageLog } from './entities/usage-log.entity';
import { UsageService } from './usage.service';

@Module({
  imports: [TypeOrmModule.forFeature([UsageLog])],
  providers: [UsageService],
  exports: [UsageService],
})
export class UsageModule {}
```

- [ ] **Step 4: Register in AppModule**

Update `backend/src/app.module.ts` — add `UsageModule`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { AiProvidersModule } from './ai-providers/ai-providers.module';
import { AuthModule } from './auth/auth.module';
import { UsageModule } from './usage/usage.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    AiProvidersModule,
    AuthModule,
    UsageModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 5: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/usage/ backend/src/app.module.ts
git commit -m "feat: add UsageLog entity and UsageService with daily counting"
```

---

### Task 8: Rewrite Module (Core Proxy)

**Files:**
- Create: `backend/src/rewrite/dto/rewrite.dto.ts`
- Create: `backend/src/rewrite/rewrite.service.ts`
- Create: `backend/src/rewrite/rewrite.controller.ts`
- Create: `backend/src/rewrite/rewrite.module.ts`

- [ ] **Step 1: Create RewriteDto**

Create `backend/src/rewrite/dto/rewrite.dto.ts`:

```typescript
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
```

- [ ] **Step 2: Create RewriteService with tone prompts**

Create `backend/src/rewrite/rewrite.service.ts`:

```typescript
import { Injectable, HttpException } from '@nestjs/common';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';

const TONE_PROMPTS: Record<string, string> = {
  simple: 'Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations.',
  natural: 'Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations.',
  polished: 'Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations.',
  concise: 'Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations.',
  technical: 'Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations.',
};

function getTranslatePrompt(targetLanguage: string): string {
  return `Translate the following text into ${targetLanguage}. If the text is already in ${targetLanguage}, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations.`;
}

@Injectable()
export class RewriteService {
  constructor(
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
    private readonly aiProvidersService: AiProvidersService,
  ) {}

  async rewrite(userId: string, text: string, tone: string, targetLanguage?: string) {
    // 1. Check subscription and daily limit
    const sub = await this.subscriptionsService.findActiveByUserId(userId);
    if (!sub || !sub.plan) {
      throw new HttpException({ error: 'No active subscription', usage_today: 0, daily_limit: 0 }, 403);
    }

    const dailyLimit = sub.plan.daily_limit;
    const usageToday = await this.usageService.countTodayByUser(userId);

    if (dailyLimit !== -1 && usageToday >= dailyLimit) {
      throw new HttpException({
        error: 'Daily limit reached',
        usage_today: usageToday,
        daily_limit: dailyLimit,
      }, 429);
    }

    // 2. Get system prompt
    const systemPrompt = tone === 'translate'
      ? getTranslatePrompt(targetLanguage || 'English')
      : TONE_PROMPTS[tone];

    if (!systemPrompt) {
      throw new HttpException({ error: `Unknown tone: ${tone}` }, 400);
    }

    // 3. Call AI provider
    const provider = await this.aiProvidersService.findDefault();

    let result: { text: string; responseTimeMs: number };
    try {
      result = await this.aiProvidersService.callProvider(provider, systemPrompt, text);
    } catch (error: any) {
      throw new HttpException({ error: `AI provider error: ${error.message}` }, 502);
    }

    // 4. Log usage
    await this.usageService.log({
      user_id: userId,
      tone,
      input_length: text.length,
      output_length: result.text.length,
      ai_provider_id: provider.id,
      response_time_ms: result.responseTimeMs,
    });

    return {
      rewritten_text: result.text,
      usage_today: usageToday + 1,
      daily_limit: dailyLimit,
    };
  }
}
```

- [ ] **Step 3: Create RewriteController**

Create `backend/src/rewrite/rewrite.controller.ts`:

```typescript
import { Controller, Post, Body, UseGuards, Request } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RewriteService } from './rewrite.service';
import { RewriteDto } from './dto/rewrite.dto';

@ApiTags('rewrite')
@Controller('rewrite')
export class RewriteController {
  constructor(private readonly rewriteService: RewriteService) {}

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Post()
  async rewrite(@Request() req: any, @Body() dto: RewriteDto) {
    return this.rewriteService.rewrite(req.user.id, dto.text, dto.tone, dto.target_language);
  }
}
```

- [ ] **Step 4: Create RewriteModule**

Create `backend/src/rewrite/rewrite.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { RewriteController } from './rewrite.controller';
import { RewriteService } from './rewrite.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';

@Module({
  imports: [SubscriptionsModule, UsageModule, AiProvidersModule],
  controllers: [RewriteController],
  providers: [RewriteService],
})
export class RewriteModule {}
```

- [ ] **Step 5: Register in AppModule**

Update `backend/src/app.module.ts` — add `RewriteModule`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { AiProvidersModule } from './ai-providers/ai-providers.module';
import { AuthModule } from './auth/auth.module';
import { UsageModule } from './usage/usage.module';
import { RewriteModule } from './rewrite/rewrite.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    AiProvidersModule,
    AuthModule,
    UsageModule,
    RewriteModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 6: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/rewrite/ backend/src/app.module.ts
git commit -m "feat: add Rewrite module — proxy with quota checking and usage logging"
```

---

### Task 9: Admin Module

**Files:**
- Create: `backend/src/admin/dto/grant-subscription.dto.ts`
- Create: `backend/src/admin/dto/update-user.dto.ts`
- Create: `backend/src/admin/admin.controller.ts`
- Create: `backend/src/admin/admin.module.ts`

- [ ] **Step 1: Create Admin DTOs**

Create `backend/src/admin/dto/grant-subscription.dto.ts`:

```typescript
import { IsUUID, IsOptional, IsDateString } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

export class GrantSubscriptionDto {
  @ApiProperty()
  @IsUUID()
  user_id: string;

  @ApiProperty()
  @IsUUID()
  plan_id: string;

  @ApiPropertyOptional()
  @IsOptional()
  @IsDateString()
  expires_at?: string;
}
```

Create `backend/src/admin/dto/update-user.dto.ts`:

```typescript
import { IsOptional, IsBoolean, IsString, IsIn } from 'class-validator';
import { ApiPropertyOptional } from '@nestjs/swagger';

export class UpdateUserDto {
  @ApiPropertyOptional()
  @IsOptional()
  @IsBoolean()
  is_active?: boolean;

  @ApiPropertyOptional({ enum: ['user', 'admin'] })
  @IsOptional()
  @IsIn(['user', 'admin'])
  role?: string;

  @ApiPropertyOptional()
  @IsOptional()
  @IsString()
  name?: string;
}
```

- [ ] **Step 2: Create AdminController**

Create `backend/src/admin/admin.controller.ts`:

```typescript
import {
  Controller, Get, Post, Patch, Delete, Body, Param, Query, UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { GrantSubscriptionDto } from './dto/grant-subscription.dto';
import { UpdateUserDto } from './dto/update-user.dto';

@ApiTags('admin')
@ApiBearerAuth()
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('admin')
@Controller('admin')
export class AdminController {
  constructor(
    private readonly usersService: UsersService,
    private readonly plansService: PlansService,
    private readonly aiProvidersService: AiProvidersService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
  ) {}

  // --- Stats ---

  @Get('stats')
  async getStats() {
    const [total_users, active_subscriptions, rewrites_today, rewrites_this_month] = await Promise.all([
      this.usersService.count(),
      this.subscriptionsService.countActive(),
      this.usageService.countToday(),
      this.usageService.countThisMonth(),
    ]);
    return { total_users, active_subscriptions, rewrites_today, rewrites_this_month };
  }

  // --- Users ---

  @Get('users')
  async listUsers(@Query('search') search?: string, @Query('page') page?: string, @Query('limit') limit?: string) {
    const result = await this.usersService.findAll({
      search,
      page: page ? parseInt(page) : 1,
      limit: limit ? parseInt(limit) : 20,
    });

    const usersWithSubs = await Promise.all(
      result.users.map(async (user) => {
        const sub = await this.subscriptionsService.findActiveByUserId(user.id);
        const usageToday = await this.usageService.countTodayByUser(user.id);
        return {
          id: user.id,
          email: user.email,
          name: user.name,
          role: user.role,
          is_active: user.is_active,
          plan: sub?.plan?.name || 'None',
          usage_today: usageToday,
          created_at: user.created_at,
        };
      }),
    );

    return { users: usersWithSubs, total: result.total };
  }

  @Get('users/:id')
  async getUser(@Param('id') id: string) {
    const user = await this.usersService.findById(id);
    const sub = await this.subscriptionsService.findActiveByUserId(id);
    const usageToday = await this.usageService.countTodayByUser(id);
    const recentUsage = await this.usageService.findRecentByUser(id);
    return { user, subscription: sub, usage_today: usageToday, recent_usage: recentUsage };
  }

  @Patch('users/:id')
  async updateUser(@Param('id') id: string, @Body() dto: UpdateUserDto) {
    return this.usersService.update(id, dto as any);
  }

  // --- Plans ---

  @Get('plans')
  async listPlans() {
    return this.plansService.findAll();
  }

  @Post('plans')
  async createPlan(@Body() body: { name: string; daily_limit: number; price_cents: number; billing_period: string }) {
    return this.plansService.create(body as any);
  }

  @Patch('plans/:id')
  async updatePlan(@Param('id') id: string, @Body() body: Partial<{ name: string; daily_limit: number; price_cents: number; billing_period: string; is_active: boolean }>) {
    return this.plansService.update(id, body as any);
  }

  @Delete('plans/:id')
  async deletePlan(@Param('id') id: string) {
    await this.plansService.softDelete(id);
    return { success: true };
  }

  // --- AI Providers ---

  @Get('ai-providers')
  async listProviders() {
    return this.aiProvidersService.findAll();
  }

  @Post('ai-providers')
  async createProvider(@Body() body: { name: string; type: string; endpoint_url: string; api_key?: string; model: string; temperature?: number }) {
    return this.aiProvidersService.create(body as any);
  }

  @Patch('ai-providers/:id')
  async updateProvider(@Param('id') id: string, @Body() body: Partial<{ name: string; type: string; endpoint_url: string; api_key: string; model: string; temperature: number; is_default: boolean; is_active: boolean }>) {
    return this.aiProvidersService.update(id, body as any);
  }

  @Delete('ai-providers/:id')
  async deleteProvider(@Param('id') id: string) {
    await this.aiProvidersService.softDelete(id);
    return { success: true };
  }

  @Post('ai-providers/:id/test')
  async testProvider(@Param('id') id: string) {
    const provider = await this.aiProvidersService.findById(id);
    if (!provider) return { success: false, error: 'Provider not found' };

    try {
      const result = await this.aiProvidersService.callProvider(provider, 'Rewrite this text to be more concise.', 'This is a test sentence to verify the connection works properly.');
      return { success: true, response: result.text, response_time_ms: result.responseTimeMs };
    } catch (error: any) {
      return { success: false, error: error.message };
    }
  }

  // --- Subscriptions ---

  @Post('subscriptions/grant')
  async grantSubscription(@Body() dto: GrantSubscriptionDto) {
    const expiresAt = dto.expires_at ? new Date(dto.expires_at) : undefined;
    const sub = await this.subscriptionsService.grant(dto.user_id, dto.plan_id, expiresAt);
    return sub;
  }
}
```

- [ ] **Step 3: Create AdminModule**

Create `backend/src/admin/admin.module.ts`:

```typescript
import { Module } from '@nestjs/common';
import { AdminController } from './admin.controller';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';

@Module({
  imports: [UsersModule, PlansModule, AiProvidersModule, SubscriptionsModule, UsageModule],
  controllers: [AdminController],
})
export class AdminModule {}
```

- [ ] **Step 4: Register in AppModule**

Update `backend/src/app.module.ts` — add `AdminModule`:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { AiProvidersModule } from './ai-providers/ai-providers.module';
import { AuthModule } from './auth/auth.module';
import { UsageModule } from './usage/usage.module';
import { RewriteModule } from './rewrite/rewrite.module';
import { AdminModule } from './admin/admin.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    AiProvidersModule,
    AuthModule,
    UsageModule,
    RewriteModule,
    AdminModule,
  ],
})
export class AppModule {}
```

- [ ] **Step 5: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/admin/ backend/src/app.module.ts
git commit -m "feat: add Admin module — users, plans, providers, subscriptions CRUD"
```

---

### Task 10: Seed Script

**Files:**
- Create: `backend/src/seed.ts`

- [ ] **Step 1: Create seed script**

Create `backend/src/seed.ts`:

```typescript
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { UsersService } from './users/users.service';
import { PlansService } from './plans/plans.service';
import { SubscriptionsService } from './subscriptions/subscriptions.service';
import { AiProvidersService } from './ai-providers/ai-providers.service';
import { AiProviderType } from './ai-providers/entities/ai-provider.entity';
import { BillingPeriod } from './plans/entities/plan.entity';
import { UserRole } from './users/entities/user.entity';
import * as bcrypt from 'bcrypt';

async function seed() {
  const app = await NestFactory.createApplicationContext(AppModule);

  const usersService = app.get(UsersService);
  const plansService = app.get(PlansService);
  const subscriptionsService = app.get(SubscriptionsService);
  const aiProvidersService = app.get(AiProvidersService);

  // 1. Create Free plan if not exists
  const existingPlans = await plansService.findAll();
  let freePlan = existingPlans.find(p => p.billing_period === BillingPeriod.NONE);
  if (!freePlan) {
    freePlan = await plansService.create({
      name: 'Free',
      daily_limit: 10,
      price_cents: 0,
      billing_period: BillingPeriod.NONE,
      is_active: true,
    });
    console.log('Created Free plan');
  } else {
    console.log('Free plan already exists');
  }

  // 2. Create admin user if not exists
  const adminEmail = process.env.ADMIN_EMAIL || 'admin@draftright.com';
  const adminPassword = process.env.ADMIN_PASSWORD || 'admin123';
  const existingAdmin = await usersService.findByEmail(adminEmail);
  if (!existingAdmin) {
    const password_hash = await bcrypt.hash(adminPassword, 10);
    const admin = await usersService.create({
      email: adminEmail,
      password_hash,
      name: 'Admin',
      role: UserRole.ADMIN,
    });
    await subscriptionsService.createFreeSubscription(admin.id, freePlan.id);
    console.log(`Created admin user: ${adminEmail}`);
  } else {
    console.log('Admin user already exists');
  }

  // 3. Create default AI provider if none exists
  const existingProviders = await aiProvidersService.findAll();
  if (existingProviders.length === 0) {
    await aiProvidersService.create({
      name: 'OpenAI',
      type: AiProviderType.OPENAI,
      endpoint_url: 'https://api.openai.com/v1/chat/completions',
      api_key: process.env.OPENAI_API_KEY || '',
      model: 'gpt-4o-mini',
      temperature: 0.3,
      is_default: true,
      is_active: true,
    });
    console.log('Created default OpenAI provider');
  } else {
    console.log('AI providers already exist');
  }

  await app.close();
  console.log('Seed complete');
}

seed().catch((err) => {
  console.error('Seed failed:', err);
  process.exit(1);
});
```

- [ ] **Step 2: Verify build and commit**

```bash
cd /opt/openAi/DraftRight/backend && npm run build
cd /opt/openAi/DraftRight
git add backend/src/seed.ts
git commit -m "feat: add seed script for free plan, admin user, and default AI provider"
```

---

### Task 11: Docker Build + Integration Test

**Files:** None (verification only)

- [ ] **Step 1: Create .env for local development**

```bash
cd /opt/openAi/DraftRight/backend
cp .env.example .env
```

Edit `.env` with real values (JWT secrets, OpenAI key, admin credentials).

- [ ] **Step 2: Start Docker Compose**

```bash
cd /opt/openAi/DraftRight
docker compose up -d postgres
```

Wait for PostgreSQL to be healthy:

```bash
docker compose ps
```

Expected: `postgres` service is `healthy`.

- [ ] **Step 3: Run seed locally**

```bash
cd /opt/openAi/DraftRight/backend
source .env && npx ts-node src/seed.ts
```

Expected: "Created Free plan", "Created admin user", "Created default OpenAI provider", "Seed complete"

- [ ] **Step 4: Start backend locally**

```bash
cd /opt/openAi/DraftRight/backend
source .env && npm run start:dev
```

Expected: "DraftRight API running on port 3000"

- [ ] **Step 5: Test auth endpoints**

Register:
```bash
curl -s -X POST http://localhost:3000/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test1234","name":"Test User"}' | jq .
```

Expected: `{ access_token, refresh_token, user: { id, email, name } }`

Login:
```bash
curl -s -X POST http://localhost:3000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test1234"}' | jq .
```

Expected: `{ access_token, refresh_token, user }`

- [ ] **Step 6: Test rewrite endpoint**

Use the access_token from login:
```bash
TOKEN="<access_token from above>"
curl -s -X POST http://localhost:3000/rewrite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"text":"this is a test message that need to be rewrite","tone":"polished"}' | jq .
```

Expected: `{ rewritten_text, usage_today: 1, daily_limit: 10 }`

- [ ] **Step 7: Test admin endpoints**

Login as admin:
```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:3000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@draftright.com","password":"admin123"}' | jq -r .access_token)

curl -s http://localhost:3000/admin/stats \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Expected: `{ total_users, active_subscriptions, rewrites_today, rewrites_this_month }`

- [ ] **Step 8: Verify Swagger docs**

Open `http://localhost:3000/api/docs` in browser.

Expected: Swagger UI with all endpoints documented.

- [ ] **Step 9: Test full Docker Compose build**

```bash
cd /opt/openAi/DraftRight
docker compose up --build -d
```

Expected: both `backend` and `postgres` containers running.

- [ ] **Step 10: Commit any fixes and final commit**

```bash
cd /opt/openAi/DraftRight
git add backend/ docker-compose.yml
git commit -m "feat: complete DraftRight Backend API — all modules, seed, Docker"
```

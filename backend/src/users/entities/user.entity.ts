import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum UserRole {
  USER = 'user',
}

export enum AuthProvider {
  LOCAL = 'local',
  GOOGLE = 'google',
  FACEBOOK = 'facebook',
  TIKTOK = 'tiktok',
  APPLE = 'apple',
}

@Entity('users')
export class User {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 255, unique: true })
  email: string;

  @Column({ type: 'varchar', length: 255, nullable: true })
  password_hash: string;

  @Column({ type: 'varchar', length: 255 })
  name: string;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @Column({ type: 'enum', enum: UserRole, default: UserRole.USER })
  role: UserRole;

  @Column({ type: 'enum', enum: AuthProvider, default: AuthProvider.LOCAL })
  auth_provider: AuthProvider;

  @Column({ type: 'varchar', length: 255, nullable: true, unique: true })
  google_id: string;

  @Column({ type: 'varchar', length: 255, nullable: true, unique: true })
  facebook_id: string;

  @Column({ type: 'varchar', length: 255, nullable: true, unique: true })
  tiktok_id: string;

  /**
   * Apple's `sub` claim from the identity token — the stable user ID
   * Apple guarantees is unique per (user, team).  Distinct from the
   * email column because Apple users can opt to receive a relay
   * address rather than expose their real email.  Set only when the
   * user signs in with Sign in with Apple.
   */
  @Column({ type: 'varchar', length: 255, nullable: true, unique: true })
  apple_id: string;

  @Column({ type: 'varchar', length: 500, nullable: true })
  avatar_url: string;

  /**
   * Stripe Customer ID (cus_XXXX) — created on first Stripe checkout, reused
   * on subsequent checkouts so we don't create duplicate Customer records.
   * NULL until the user makes their first Stripe payment.
   */
  @Column({ type: 'varchar', length: 255, nullable: true })
  stripe_customer_id: string | null;

  @Column({ type: 'boolean', default: false })
  email_verified: boolean;

  @Column({ type: 'varchar', length: 6, nullable: true })
  email_verification_code: string | null;

  @Column({ type: 'timestamptz', nullable: true })
  email_verification_expires: Date | null;

  @Column({ type: 'varchar', length: 6, nullable: true })
  password_reset_code: string | null;

  @Column({ type: 'timestamptz', nullable: true })
  password_reset_expires: Date | null;

  @Column({ type: 'int', default: 0 })
  password_reset_attempts: number;

  @Column({ type: 'varchar', length: 64, nullable: true })
  lemonsqueezy_customer_id: string | null;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}

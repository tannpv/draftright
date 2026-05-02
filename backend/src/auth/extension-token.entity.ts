import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  JoinColumn,
  ManyToOne,
  PrimaryGeneratedColumn,
} from 'typeorm';
import { User } from '../users/entities/user.entity';

@Entity('extension_tokens')
@Index(['user_id'])
export class ExtensionToken {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  @ManyToOne(() => User, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'user_id' })
  user: User;

  @Column({ type: 'char', length: 64 })
  token_hash: string;

  @Column({ type: 'text', array: true, default: () => "ARRAY['rewrite']" })
  scopes: string[];

  @Column({ type: 'uuid' })
  device_id: string;

  @Column({ type: 'varchar', length: 64, default: 'mobile' })
  device_name: string;

  @Column({ type: 'timestamp', nullable: true })
  last_used_at: Date | null;

  @CreateDateColumn({ type: 'timestamp' })
  created_at: Date;

  @Column({ type: 'timestamp', nullable: true })
  revoked_at: Date | null;
}

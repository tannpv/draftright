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

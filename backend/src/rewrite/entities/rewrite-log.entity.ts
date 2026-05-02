import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, Index,
} from 'typeorm';

@Entity('rewrite_logs')
@Index(['tone', 'created_at'])
export class RewriteLog {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 20 })
  tone: string;

  @Column({ type: 'text' })
  input_text: string;

  @Column({ type: 'text' })
  output_text: string;

  @Column({ type: 'varchar', length: 100 })
  model: string;

  @Column({ type: 'varchar', length: 20 })
  provider_type: string;

  @Column({ type: 'int' })
  response_time_ms: number;

  @Column({ type: 'varchar', length: 20, default: 'pending' })
  quality: string; // pending, approved, rejected — for human review before fine-tuning

  @CreateDateColumn()
  created_at: Date;
}

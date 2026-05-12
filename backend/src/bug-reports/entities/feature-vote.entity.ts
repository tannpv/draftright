import {
  Entity, PrimaryGeneratedColumn, Column, Index, Unique, CreateDateColumn,
} from 'typeorm';

/**
 * One row per (feature request, user) upvote. The UNIQUE constraint
 * enforces one vote per user per feature; `bug_reports.vote_count` is
 * always recomputed as COUNT(*) of these rows for the feature.
 */
@Entity('feature_votes')
@Unique('feature_votes_feature_user_uq', ['feature_id', 'user_id'])
@Index('feature_votes_feature_idx', ['feature_id'])
export class FeatureVote {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid', name: 'feature_id' })
  feature_id: string;

  @Column({ type: 'uuid', name: 'user_id' })
  user_id: string;

  @CreateDateColumn({ name: 'created_at' })
  created_at: Date;
}

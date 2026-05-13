import type { ReactNode } from 'react';

export interface BoardRow {
  id: string;
  title: string | null;
  description: string;
  target_platform: string | null;
  status: string;
  vote_count: number;
  source: string;
  created_at: string;
  viewerHasVoted: boolean;
}

interface Props {
  row: BoardRow;
  isSignedIn: boolean;
  onVote: (id: string) => void;
}

const PLATFORM_STYLES: Record<string, string> = {
  playground: 'bg-purple-900/40 border-purple-700/60',
  mobile: 'bg-emerald-900/40 border-emerald-700/60',
  windows: 'bg-sky-900/40 border-sky-700/60',
  mac: 'bg-zinc-700/40 border-zinc-500/60',
  linux: 'bg-amber-900/40 border-amber-700/60',
};

const PLATFORM_LABEL: Record<string, string> = {
  playground: 'Playground', mobile: 'Mobile', windows: 'Windows', mac: 'macOS', linux: 'Linux',
};

const STATUS_LABEL: Record<string, string> = {
  new: 'open', open: 'open', reviewing: 'reviewing',
  planned: 'planned', in_progress: 'in progress',
  fix_proposed: 'in progress', resolved: 'done', done: 'done',
  declined: 'declined', wont_fix: 'declined',
};

const STATUS_STYLES: Record<string, string> = {
  open: 'text-slate-400 border-slate-600',
  reviewing: 'text-slate-300 border-slate-500',
  planned: 'text-sky-400 border-sky-500',
  'in progress': 'text-amber-400 border-amber-500',
  done: 'text-emerald-400 border-emerald-500',
  declined: 'text-rose-400 border-rose-500',
};

function statusKey(raw: string): string { return STATUS_LABEL[raw] ?? raw; }

export default function FeedbackBoardCard({ row, isSignedIn, onVote }: Props) {
  const plat = row.target_platform ?? '';
  const platLabel = PLATFORM_LABEL[plat] ?? plat;
  const platStyle = PLATFORM_STYLES[plat] ?? 'bg-zinc-800 border-zinc-700';
  const status = statusKey(row.status);
  const statusStyle = STATUS_STYLES[status] ?? STATUS_STYLES.open;

  const voteTitle = isSignedIn
    ? row.viewerHasVoted ? 'Remove your vote' : 'Upvote this request'
    : 'Sign in to vote';
  const voteClasses = [
    'flex flex-col items-center justify-center min-w-[56px] rounded-lg border px-3 py-2 transition select-none',
    isSignedIn ? 'cursor-pointer' : 'cursor-not-allowed opacity-60',
    row.viewerHasVoted
      ? 'bg-sky-900/40 border-sky-500 text-sky-200'
      : 'bg-zinc-800/60 border-zinc-700 text-zinc-300 hover:border-sky-500',
  ].join(' ');

  return (
    <li className="flex gap-4 rounded-xl border border-zinc-700 bg-zinc-900/60 p-4">
      <button
        type="button"
        aria-label={voteTitle}
        title={voteTitle}
        disabled={!isSignedIn}
        onClick={() => onVote(row.id)}
        className={voteClasses}>
        <span className="text-sm leading-none">▲</span>
        <span className="text-base font-bold tabular-nums">{row.vote_count}</span>
      </button>
      <div className="min-w-0 flex-1">
        <h3 className="text-base font-semibold text-zinc-100">{row.title ?? '(untitled)'}</h3>
        <p className="mt-1 line-clamp-3 text-sm text-zinc-400">{row.description}</p>
        <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
          {plat && (
            <Badge className={`${platStyle} text-zinc-100`}>{platLabel}</Badge>
          )}
          <Badge className={`${statusStyle} bg-transparent`}>{status}</Badge>
          <Badge className="ml-auto bg-transparent border-zinc-700 text-zinc-500">via {row.source}</Badge>
        </div>
      </div>
    </li>
  );
}

function Badge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={`rounded-full border px-2 py-0.5 ${className ?? ''}`}>{children}</span>
  );
}

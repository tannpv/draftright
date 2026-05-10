import { useState, useEffect, useCallback } from 'react';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

const PLATFORMS = ['mac', 'windows', 'linux', 'android', 'ios'] as const;
const CHANNELS = ['direct', 'store'] as const;
const STATUSES = ['not_submitted', 'in_review', 'approved', 'rejected', 'n/a'] as const;

type Platform = typeof PLATFORMS[number];
type Channel = typeof CHANNELS[number];

interface Release {
  platform: string;
  channel: string;
  version: string;
  download_url: string;
  release_notes: string;
  required: boolean;
  enabled: boolean;
  updated_at: string;
}

interface Policy {
  platform: string;
  preferred: string;
  store_status: string;
  notes: string;
  updated_at: string;
}

interface PlatformBundle {
  policy: Policy | null;
  channels: Record<string, Release | null>;
}

const PLATFORM_LABELS: Record<Platform, string> = {
  mac: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
};

const STATUS_COLORS: Record<string, string> = {
  approved: 'bg-[#13deb9]/15 text-[#13deb9]',
  in_review: 'bg-[#ffae1f]/15 text-[#ffae1f]',
  rejected: 'bg-[#fa896b]/15 text-[#fa896b]',
  not_submitted: 'bg-[#7c8fac]/15 text-[#7c8fac]',
  'n/a': 'bg-[#7c8fac]/15 text-[#7c8fac]',
};

export default function VersionsPage() {
  const [data, setData] = useState<Record<Platform, PlatformBundle> | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const result = await apiFetch('/admin/releases') as Record<Platform, PlatformBundle>;
      setData(result);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function saveChannel(platform: string, channel: string, body: Partial<Release>) {
    try {
      await apiFetch('/admin/releases', {
        method: 'POST',
        body: JSON.stringify({ platform, channel, ...body }),
      });
      setToast({ message: `Saved ${platform}/${channel}`, type: 'success' });
      await load();
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Save failed', type: 'error' });
    }
  }

  async function deleteChannel(platform: string, channel: string) {
    if (!confirm(`Delete ${platform}/${channel} row?`)) return;
    try {
      await apiFetch(`/admin/releases/${platform}/${channel}`, { method: 'DELETE' });
      setToast({ message: `Deleted ${platform}/${channel}`, type: 'success' });
      await load();
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Delete failed', type: 'error' });
    }
  }

  async function savePolicy(platform: string, body: Partial<Policy>) {
    try {
      await apiFetch('/admin/release-policies', {
        method: 'POST',
        body: JSON.stringify({ platform, ...body }),
      });
      setToast({ message: `Updated policy for ${platform}`, type: 'success' });
      await load();
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Save failed', type: 'error' });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-[#eaeff4]">App Versions</h1>
        <button
          onClick={load}
          className="px-3 py-1.5 text-sm bg-[#5d87ff] text-white rounded hover:bg-[#5d87ff]/90"
        >
          Refresh
        </button>
      </div>

      <p className="text-sm text-[#7c8fac]">
        Manage which channel (Store or Direct download) each platform&rsquo;s auto-updater surfaces.
        Flip <strong>Preferred</strong> the moment a store approves us &mdash; every running app picks
        up the change on its next <code className="text-[#49beff]">/updates/latest</code> poll.
      </p>

      {error && <div className="p-3 bg-[#fa896b]/10 text-[#fa896b] rounded text-sm">{error}</div>}
      {loading && <div className="text-[#7c8fac] text-sm">Loading&hellip;</div>}

      {data && PLATFORMS.map(platform => (
        <PlatformCard
          key={platform}
          platform={platform}
          bundle={data[platform]}
          onSaveChannel={(channel, body) => saveChannel(platform, channel, body)}
          onDeleteChannel={(channel) => deleteChannel(platform, channel)}
          onSavePolicy={(body) => savePolicy(platform, body)}
        />
      ))}

      {toast && (
        <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />
      )}
    </div>
  );
}

interface PlatformCardProps {
  platform: Platform;
  bundle: PlatformBundle;
  onSaveChannel: (channel: string, body: Partial<Release>) => void;
  onDeleteChannel: (channel: string) => void;
  onSavePolicy: (body: Partial<Policy>) => void;
}

function PlatformCard({ platform, bundle, onSaveChannel, onDeleteChannel, onSavePolicy }: PlatformCardProps) {
  const policy = bundle.policy ?? {
    platform,
    preferred: 'direct',
    store_status: 'not_submitted',
    notes: '',
    updated_at: '',
  };

  return (
    <div className="bg-[#2a3547] border border-[#333f55] rounded-lg p-5">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-[#eaeff4]">{PLATFORM_LABELS[platform]}</h2>
        <span className={`text-xs px-2 py-1 rounded ${STATUS_COLORS[policy.store_status] ?? STATUS_COLORS.not_submitted}`}>
          {policy.store_status.replace('_', ' ')}
        </span>
      </div>

      {/* Policy controls */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-5 p-3 bg-[#202936] rounded">
        <div>
          <label className="block text-xs text-[#7c8fac] mb-1">Preferred channel</label>
          <select
            value={policy.preferred}
            onChange={(e) => onSavePolicy({ preferred: e.target.value })}
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded"
          >
            {CHANNELS.map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-[#7c8fac] mb-1">Store status</label>
          <select
            value={policy.store_status}
            onChange={(e) => onSavePolicy({ store_status: e.target.value })}
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded"
          >
            {STATUSES.map(s => <option key={s} value={s}>{s}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-[#7c8fac] mb-1">Policy notes</label>
          <input
            type="text"
            defaultValue={policy.notes}
            onBlur={(e) => { if (e.target.value !== policy.notes) onSavePolicy({ notes: e.target.value }); }}
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded"
            placeholder="e.g. Apple rejected for X, resubmitted Y"
          />
        </div>
      </div>

      {/* Channel rows */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {CHANNELS.map(channel => (
          <ChannelEditor
            key={channel}
            platform={platform}
            channel={channel}
            row={bundle.channels[channel]}
            isPreferred={policy.preferred === channel}
            onSave={(body) => onSaveChannel(channel, body)}
            onDelete={() => onDeleteChannel(channel)}
          />
        ))}
      </div>
    </div>
  );
}

interface ChannelEditorProps {
  platform: string;
  channel: Channel;
  row: Release | null;
  isPreferred: boolean;
  onSave: (body: Partial<Release>) => void;
  onDelete: () => void;
}
// `platform` prop kept for clarity / future expansion (not used yet).

function ChannelEditor({ channel, row, isPreferred, onSave, onDelete }: ChannelEditorProps) {
  const [version, setVersion] = useState(row?.version ?? '');
  const [url, setUrl] = useState(row?.download_url ?? '');
  const [notes, setNotes] = useState(row?.release_notes ?? '');
  const [required, setRequired] = useState(row?.required ?? false);
  const [enabled, setEnabled] = useState(row?.enabled ?? true);
  const [editing, setEditing] = useState(false);

  // Keep local state in sync if parent reloads.
  useEffect(() => {
    setVersion(row?.version ?? '');
    setUrl(row?.download_url ?? '');
    setNotes(row?.release_notes ?? '');
    setRequired(row?.required ?? false);
    setEnabled(row?.enabled ?? true);
    setEditing(false);
  }, [row]);

  const exists = !!row;

  return (
    <div className={`p-3 rounded border ${isPreferred ? 'border-[#5d87ff] bg-[#5d87ff]/5' : 'border-[#333f55] bg-[#202936]'}`}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-[#eaeff4] uppercase">{channel}</h3>
          {isPreferred && <span className="text-[10px] px-1.5 py-0.5 bg-[#5d87ff] text-white rounded">PREFERRED</span>}
          {exists && !enabled && <span className="text-[10px] px-1.5 py-0.5 bg-[#7c8fac]/30 text-[#7c8fac] rounded">DISABLED</span>}
        </div>
        {!editing && exists && (
          <button onClick={() => setEditing(true)} className="text-xs text-[#5d87ff] hover:underline">Edit</button>
        )}
        {!editing && !exists && (
          <button onClick={() => setEditing(true)} className="text-xs text-[#5d87ff] hover:underline">+ Add</button>
        )}
      </div>

      {!editing && exists && (
        <div className="text-xs text-[#7c8fac] space-y-1">
          <div><span className="text-[#eaeff4]">v{row?.version}</span></div>
          <div className="truncate font-mono text-[10px]">{row?.download_url}</div>
          {row?.release_notes && <div className="italic line-clamp-2">{row.release_notes}</div>}
          {row?.required && <div className="text-[#ffae1f]">Required update</div>}
        </div>
      )}

      {!editing && !exists && (
        <div className="text-xs text-[#7c8fac] italic">No URL configured for this channel.</div>
      )}

      {editing && (
        <div className="space-y-2">
          <input
            type="text"
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder="Version (e.g. 2.1.9)"
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded font-mono"
          />
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={channel === 'store' ? 'apps.apple.com/...' : 'draftright.info/downloads/...'}
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded font-mono"
          />
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Release notes (optional)"
            rows={2}
            className="w-full px-2 py-1.5 bg-[#2a3547] border border-[#333f55] text-[#eaeff4] text-sm rounded"
          />
          <div className="flex items-center gap-3 text-xs text-[#7c8fac]">
            <label className="flex items-center gap-1">
              <input type="checkbox" checked={required} onChange={(e) => setRequired(e.target.checked)} />
              Required
            </label>
            <label className="flex items-center gap-1">
              <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
              Enabled
            </label>
          </div>
          <div className="flex items-center gap-2 pt-1">
            <button
              onClick={() => onSave({ version, download_url: url, release_notes: notes, required, enabled })}
              disabled={!version || !url}
              className="px-2.5 py-1 bg-[#5d87ff] text-white text-xs rounded disabled:opacity-50"
            >
              Save
            </button>
            <button
              onClick={() => setEditing(false)}
              className="px-2.5 py-1 bg-[#333f55] text-[#eaeff4] text-xs rounded"
            >
              Cancel
            </button>
            {exists && (
              <button
                onClick={onDelete}
                className="ml-auto px-2.5 py-1 text-[#fa896b] text-xs hover:underline"
              >
                Delete
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

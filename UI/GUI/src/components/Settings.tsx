import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import type { ActivityNote } from '../types';

interface SettingsProps {
  fetchRuntime: (path: string, init?: RequestInit) => Promise<Response>;
  pushActivity: (kind: ActivityNote['kind'], title: string, body: string, meta?: string) => void;
}

type RuntimeConfig = Record<string, any>;

type ModelKind = 'chat' | 'vision' | 'embedding' | 'transcription' | 'image' | 'tts' | 'reranker';

const MODEL_KINDS: Array<{ id: ModelKind; label: string; detail: string }> = [
  { id: 'chat', label: 'Chat', detail: 'Agent reasoning and replies' },
  { id: 'vision', label: 'Vision', detail: 'Image understanding' },
  { id: 'embedding', label: 'Embedding', detail: 'RAG and memory vectors' },
  { id: 'transcription', label: 'Transcription', detail: 'Audio to text' },
  { id: 'image', label: 'Image generation', detail: 'Image creation' },
  { id: 'tts', label: 'Text to speech', detail: 'Speech synthesis' },
  { id: 'reranker', label: 'Reranker', detail: 'RAG result ranking' },
];

function asObject(value: unknown): Record<string, any> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, any> : {};
}

function legacyModel(config: RuntimeConfig, kind: ModelKind): string {
  if (kind === 'chat') return String(asObject(config.llm_provider).model || '');
  if (kind === 'vision') return String(asObject(config.multimodal).image_model || '');
  if (kind === 'embedding') return String(asObject(config.embedding).model || '');
  if (kind === 'transcription') return String(asObject(config.multimodal).transcription_model || '');
  if (kind === 'image') return String(asObject(config.image_generation).model || '');
  if (kind === 'tts') return String(asObject(config.tts).model || '');
  return '';
}

function legacyEndpoint(config: RuntimeConfig, kind: ModelKind): Record<string, any> {
  if (kind === 'chat') {
    const llm = asObject(config.llm_provider);
    return { provider: llm.name || '', api_base: llm.base_url || '', protocol: llm.protocol || '', api_key: llm.api_key || '' };
  }
  if (kind === 'embedding') return asObject(config.embedding);
  if (kind === 'vision' || kind === 'transcription') return asObject(config.multimodal);
  if (kind === 'image') return asObject(config.image_generation);
  if (kind === 'tts') return asObject(config.tts);
  return {};
}

function cloneConfig(config: RuntimeConfig): RuntimeConfig {
  return JSON.parse(JSON.stringify(config || {})) as RuntimeConfig;
}

function prettyConfig(config: RuntimeConfig): string {
  return JSON.stringify(config, null, 2);
}

export function Settings({ fetchRuntime, pushActivity }: SettingsProps) {
  const [config, setConfig] = useState<RuntimeConfig>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [advancedText, setAdvancedText] = useState('');
  const errorRef = useRef<HTMLDivElement | null>(null);

  const models = useMemo(() => asObject(asObject(config.models).active), [config]);
  const endpoints = useMemo(() => asObject(asObject(config.models).endpoints), [config]);

  useEffect(() => {
    void loadConfig();
  }, []);

  useEffect(() => {
    if (error) errorRef.current?.focus();
  }, [error]);

  async function loadConfig() {
    setLoading(true);
    setError('');
    try {
      const response = await fetchRuntime('/v1/config');
      if (!response.ok) throw new Error(`config ${response.status}`);
      const data = await response.json() as RuntimeConfig;
      setConfig(data);
      setAdvancedText(prettyConfig(data));
    } catch (loadError) {
      const message = String(loadError);
      setError(message);
      pushActivity('error', 'Load config failed', message);
    } finally {
      setLoading(false);
    }
  }

  function updateConfig(mutator: (next: RuntimeConfig) => void) {
    setConfig((previous) => {
      const next = cloneConfig(previous);
      mutator(next);
      setAdvancedText(prettyConfig(next));
      return next;
    });
  }

  function updateModel(kind: ModelKind, value: string) {
    updateConfig((next) => {
      next.models = asObject(next.models);
      next.models.active = asObject(next.models.active);
      next.models.active[kind] = value;
    });
  }

  function updateEndpoint(kind: ModelKind, field: string, value: string) {
    updateConfig((next) => {
      next.models = asObject(next.models);
      next.models.endpoints = asObject(next.models.endpoints);
      next.models.endpoints[kind] = asObject(next.models.endpoints[kind]);
      next.models.endpoints[kind][field] = value;
    });
  }

  function updateNested(section: string, field: string, value: string | number | boolean) {
    updateConfig((next) => {
      next[section] = asObject(next[section]);
      next[section][field] = value;
    });
  }

  async function saveConfig(event?: FormEvent) {
    event?.preventDefault();
    setError('');
    const chatModel = String(models.chat || legacyModel(config, 'chat')).trim();
    const chatProvider = String(asObject(endpoints.chat).provider || legacyEndpoint(config, 'chat').provider || '').trim();
    if (!chatModel || !chatProvider) {
      setError('Chat model and provider are required before saving.');
      return;
    }
    setSaving(true);
    try {
      const response = await fetchRuntime('/v1/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      if (!response.ok) throw new Error(`save config ${response.status}`);
      const saved = await response.json() as RuntimeConfig;
      setConfig(saved);
      setAdvancedText(prettyConfig(saved));
      pushActivity('status', 'Config saved', 'Changes are active for subsequent requests. Sensitive values remain hidden.');
    } catch (saveError) {
      const message = String(saveError);
      setError(message);
      pushActivity('error', 'Save config failed', message);
    } finally {
      setSaving(false);
    }
  }

  function applyAdvanced() {
    try {
      const parsed = JSON.parse(advancedText) as RuntimeConfig;
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Configuration must be a JSON object.');
      setConfig(parsed);
      setError('');
    } catch (parseError) {
      setError(`Advanced JSON: ${String(parseError)}`);
    }
  }

  function exportConfig() {
    const blob = new Blob([prettyConfig(config)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'luckyagent-config-redacted.json';
    link.click();
    URL.revokeObjectURL(url);
    pushActivity('status', 'Config exported', 'Export contains no API keys, tokens, or authorization headers.');
  }

  function importConfig(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result || '')) as RuntimeConfig;
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Import must contain a JSON object.');
        setConfig(parsed);
        setAdvancedText(prettyConfig(parsed));
        setError('');
      } catch (importError) {
        setError(`Import failed: ${String(importError)}`);
      }
    };
    reader.readAsText(file);
    event.target.value = '';
  }

  if (loading) {
    return <section className="settings-panel" aria-busy="true"><div className="panel-body">Loading configuration…</div></section>;
  }

  return (
    <form className="settings-panel" onSubmit={saveConfig}>
      <div className="panel-head">
        <div>
          <h2>Configuration Center</h2>
          <p className="settings-desc">Local settings are applied to future requests. Existing API keys are never returned to this browser.</p>
        </div>
        <div className="panel-actions">
          <button className="ghost" type="button" onClick={() => void loadConfig()}>Reload</button>
          <button className="ghost" type="button" onClick={exportConfig}>Export safe copy</button>
          <label className="ghost import-button">Import<input type="file" accept="application/json" onChange={importConfig} /></label>
          <button className="primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save and apply'}</button>
        </div>
      </div>

      <div ref={errorRef} className="settings-error" role="alert" tabIndex={-1} hidden={!error}>{error}</div>
      <p className="settings-live" role="status">{saving ? 'Saving configuration. Please wait.' : ''}</p>

      <section className="settings-section" aria-labelledby="models-heading">
        <h3 id="models-heading">Model selection</h3>
        <p className="settings-desc">Each purpose can use a different provider, endpoint, protocol, and credential. Empty secret fields preserve the existing secret.</p>
        <div className="model-grid">
          {MODEL_KINDS.map(({ id, label, detail }) => {
            const endpoint = asObject(endpoints[id]);
            const fallback = legacyEndpoint(config, id);
            const modelID = String(models[id] ?? legacyModel(config, id));
            const provider = String(endpoint.provider ?? fallback.provider ?? '');
            const base = String(endpoint.api_base ?? fallback.api_base ?? '');
            const protocol = String(endpoint.protocol ?? fallback.protocol ?? '');
            return (
              <fieldset className="model-card" key={id}>
                <legend>{label}</legend>
                <p>{detail}</p>
                <label htmlFor={`model-${id}`}>Model ID</label>
                <input id={`model-${id}`} name={`model-${id}`} value={modelID} onChange={(event) => updateModel(id, event.target.value)} spellCheck={false} />
                <details>
                  <summary>Connection settings</summary>
                  <label htmlFor={`provider-${id}`}>Provider</label>
                  <input id={`provider-${id}`} name={`provider-${id}`} value={provider} onChange={(event) => updateEndpoint(id, 'provider', event.target.value)} spellCheck={false} />
                  <label htmlFor={`base-${id}`}>API base</label>
                  <input id={`base-${id}`} name={`base-${id}`} value={base} onChange={(event) => updateEndpoint(id, 'api_base', event.target.value)} spellCheck={false} />
                  <label htmlFor={`key-${id}`}>New API key</label>
                  <input id={`key-${id}`} name={`key-${id}`} type="password" autoComplete="off" placeholder="Leave blank to keep current key" onChange={(event) => updateEndpoint(id, 'api_key', event.target.value)} />
                  <label htmlFor={`protocol-${id}`}>Protocol</label>
                  <input id={`protocol-${id}`} name={`protocol-${id}`} value={protocol} onChange={(event) => updateEndpoint(id, 'protocol', event.target.value)} placeholder="chat_completions or responses" spellCheck={false} />
                </details>
              </fieldset>
            );
          })}
        </div>
      </section>

      <section className="settings-section settings-grid" aria-label="Core agent settings">
        <div className="setting-item"><label htmlFor="agent-timeout"><span>Agent timeout (seconds)</span><input id="agent-timeout" type="number" min="1" value={Number(asObject(config.agent).timeout_seconds || 60)} onChange={(event) => updateNested('agent', 'timeout_seconds', Number(event.target.value))} /></label></div>
        <div className="setting-item"><label htmlFor="agent-iterations"><span>Maximum iterations</span><input id="agent-iterations" type="number" min="1" value={Number(asObject(config.agent).max_iterations || 10)} onChange={(event) => updateNested('agent', 'max_iterations', Number(event.target.value))} /></label></div>
        <div className="setting-item"><label htmlFor="server-address"><span>Local server address</span><input id="server-address" value={String(asObject(config.server).addr || '')} onChange={(event) => updateNested('server', 'addr', event.target.value)} spellCheck={false} /></label></div>
      </section>

      <details className="advanced-config">
        <summary>Advanced JSON configuration</summary>
        <p>Use this for gateway, RAG, memory, autonomy, hooks, and other advanced settings. Save applies the parsed configuration.</p>
        <label htmlFor="advanced-json">Configuration JSON</label>
        <textarea id="advanced-json" value={advancedText} onChange={(event) => setAdvancedText(event.target.value)} spellCheck={false} rows={18} />
        <button className="ghost" type="button" onClick={applyAdvanced}>Use JSON in form</button>
      </details>
    </form>
  );
}

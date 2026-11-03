import {useState, type FormEvent} from 'react';
import {tracker} from '../wailsjs/go/models';

type Definition = tracker.PropertyDefinition;

function ReminderForm({settings, busy, save, preview}: {
    settings: tracker.Settings; busy: boolean; save: (s: tracker.Settings) => Promise<void>; preview: (running: boolean) => Promise<void>;
}) {
    const [runningEnabled, setRunningEnabled] = useState(settings.reminders.runningEnabled);
    const [idleEnabled, setIdleEnabled] = useState(settings.reminders.idleEnabled);
    const [runningMinutes, setRunningMinutes] = useState(String(settings.reminders.runningMinutes));
    const [idleMinutes, setIdleMinutes] = useState(String(settings.reminders.idleMinutes));
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save(Object.assign(new tracker.Settings(), settings, {reminders: {
            runningEnabled, idleEnabled, runningMinutes: Number(runningMinutes), idleMinutes: Number(idleMinutes),
        }}));
    };
    return <section className="panel">
        <div className="section-heading"><div><h2>Timer reminders</h2><p>Small corner messages, even while TimeManager is minimized to the tray.</p></div></div>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <div className="form-grid">
                <div>
                    <label className="check"><input type="checkbox" checked={runningEnabled} onChange={e => setRunningEnabled(e.target.checked)}/>Remind while a timer is running</label>
                    <label>Running reminder interval (minutes)<input required type="number" min="1" step="1" value={runningMinutes} onChange={e => setRunningMinutes(e.target.value)}/></label>
                    <button type="button" onClick={() => void preview(true)}>Preview running reminder</button>
                </div>
                <div>
                    <label className="check"><input type="checkbox" checked={idleEnabled} onChange={e => setIdleEnabled(e.target.checked)}/>Remind when no timer is running</label>
                    <label>Idle reminder interval (minutes)<input required type="number" min="1" step="1" value={idleMinutes} onChange={e => setIdleMinutes(e.target.value)}/></label>
                    <button type="button" onClick={() => void preview(false)}>Preview idle reminder</button>
                </div>
            </div>
            <p className="hint">Reminders repeat at the saved intervals (positive whole minutes). They appear at the primary screen's bottom-right, above its taskbar, without stealing keyboard focus. Close them manually or wait 15 seconds. Open TimeManager restores the app without changing your timer.</p>
            <p className="hint">The countdown restarts when the app opens, a timer starts/stops/switches, or reminder settings change. Reminders run only while the app is open. After sleep, only one overdue reminder is shown.</p>
            <div className="dialog-actions"><button className="primary">Save reminders</button></div>
        </fieldset></form>
    </section>;
}

function PropertyEditor({initial, busy, save, close}: {
    initial: Definition; busy: boolean; save: (d: Definition, done: () => void) => Promise<void>; close: () => void;
}) {
    const [name, setName] = useState(initial.name);
    const [type, setType] = useState(initial.type);
    const [options, setOptions] = useState(initial.options.join('\n'));
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save({id: initial.id, name, type, options: type === 'dropdown' ? options.split('\n') : []}, close);
    };
    return <form className="property-editor" onSubmit={submit}><fieldset disabled={busy}>
        <h3>{initial.id ? 'Edit property' : 'New property'}</h3>
        <div className="form-grid">
            <label>Property name<input required autoFocus value={name} onChange={e => setName(e.target.value)}/></label>
            <label>Property type<select value={type} disabled={!!initial.id} onChange={e => setType(e.target.value)}>
                <option value="text">Text</option><option value="number">Number</option>
                <option value="checkbox">Checkbox</option><option value="dropdown">Dropdown</option>
            </select></label>
        </div>
        {type === 'dropdown' && <label>Dropdown options (one per line)<textarea required rows={4} value={options} onChange={e => setOptions(e.target.value)}/></label>}
        <p className="hint">Existing property types cannot be changed. Dropdown options used by projects must be cleared or changed on those projects before removing the option.</p>
        <div className="dialog-actions"><button type="button" onClick={close}>Cancel</button><button className="primary">Save property</button></div>
    </fieldset></form>;
}

export default function SettingsView({settings, definitions, busy, saveSettings, saveProperty, removeProperty, previewReminder}: {
    settings: tracker.Settings; definitions: Definition[]; busy: boolean;
    saveSettings: (s: tracker.Settings) => Promise<void>;
    saveProperty: (d: Definition, done: () => void) => Promise<void>;
    removeProperty: (d: Definition) => void;
    previewReminder: (running: boolean) => Promise<void>;
}) {
    const [editing, setEditing] = useState<Definition | null>(null);
    return <div className="settings-panels">
        <section className="panel">
            <div className="section-heading"><div><h2>Appearance</h2><p>Your preference is saved on this computer.</p></div></div>
            <label className="appearance-control">Color theme<select value={settings.theme} disabled={busy} onChange={e => void saveSettings(Object.assign(new tracker.Settings(), settings, {theme: e.target.value}))}>
                <option value="light">Light</option><option value="dark">Dark</option><option value="system">Follow Windows / system</option>
            </select></label>
            <p className="hint">System mode follows the operating system's color preference and updates when it changes.</p>
        </section>
        <ReminderForm key={JSON.stringify(settings.reminders)} settings={settings} busy={busy} save={saveSettings} preview={previewReminder}/>
        <section className="panel">
            <div className="section-heading"><div><h2>Project properties</h2><p>Shared optional fields, filled in separately for each project and subproject.</p></div>
                <button className="primary" disabled={busy || !!editing} onClick={() => setEditing({id: '', name: '', type: 'text', options: []})}>Add property</button></div>
            {!definitions.length && !editing && <p className="hint">Add fields such as Cost center, Hourly rate, Billable, or Status. Values appear in the project editor and Excel exports.</p>}
            {definitions.length > 0 && <div className="table-scroll"><table className="project-table"><thead><tr><th>Property</th><th>Type</th><th>Options</th><th>Actions</th></tr></thead>
                <tbody>{definitions.map(def => <tr key={def.id}><td><strong>{def.name}</strong></td><td>{def.type}</td><td>{def.options.join(', ') || '-'}</td>
                    <td className="row-actions"><button disabled={busy || !!editing} aria-label={`Edit property ${def.name}`} onClick={() => setEditing(def)}>Edit</button>
                        <button disabled={busy || !!editing} className="danger" aria-label={`Delete property ${def.name}`} onClick={() => removeProperty(def)}>Delete</button></td></tr>)}</tbody>
            </table></div>}
            {editing && <PropertyEditor key={editing.id} initial={editing} busy={busy} save={saveProperty} close={() => setEditing(null)}/>}
            <p className="hint">Deleting a definition requires confirmation and removes its values from all projects, including archived projects. Excel uses current property names and values, not historical snapshots.</p>
        </section>
    </div>;
}

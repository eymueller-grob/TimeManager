import {BUSY_REASON, Button, Check, whyDisabled} from './Controls';
import {Input} from './components/ui/input';
import {Textarea} from './components/ui/textarea';
import {Label} from './components/ui/label';
import {NativeSelect, NativeSelectOption} from './components/ui/native-select';
import {useState, type FormEvent} from 'react';
import {tracker} from '../wailsjs/go/models';
import {Card} from './components/ui/card';
import {ButtonGroup} from './components/ui/button-group';
import {Table, TableHeader, TableBody, TableRow, TableHead, TableCell} from './components/ui/table';

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
    return <Card className="panel">
        <div className="section-heading"><div><h2>Timer reminders</h2><p>Small corner messages, even while TimeManager is minimized to the tray.</p></div></div>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <div className="form-grid">
                <div>
                    <Label className="check"><Check  checked={runningEnabled} onCheckedChange={checked => setRunningEnabled(checked)}/>Remind while a timer is running</Label>
                    <Label>Running reminder interval (minutes)<Input required type="number" min="1" step="1" value={runningMinutes} onChange={e => setRunningMinutes(e.target.value)}/></Label>
                    <Button type="button" onClick={() => void preview(true)}>Preview running reminder</Button>
                </div>
                <div>
                    <Label className="check"><Check  checked={idleEnabled} onCheckedChange={checked => setIdleEnabled(checked)}/>Remind when no timer is running</Label>
                    <Label>Idle reminder interval (minutes)<Input required type="number" min="1" step="1" value={idleMinutes} onChange={e => setIdleMinutes(e.target.value)}/></Label>
                    <Button type="button" onClick={() => void preview(false)}>Preview idle reminder</Button>
                </div>
            </div>
            <p className="hint">Reminders repeat at the saved intervals (positive whole minutes). They appear at the primary screen's bottom-right, above its taskbar, without stealing keyboard focus. Close them manually or wait 15 seconds. Open TimeManager restores the app without changing your timer.</p>
            <p className="hint">The countdown restarts when the app opens, a timer starts/stops/switches, or reminder settings change. Reminders run only while the app is open. After sleep, only one overdue reminder is shown.</p>
            <div className="dialog-actions"><Button className="primary">Save reminders</Button></div>
        </fieldset></form>
    </Card>;
}

function PropertyEditor({initial, busy, save, close}: {
    initial: Definition; busy: boolean; save: (d: Definition, done: () => void) => Promise<void>; close: () => void;
}) {
    const [name, setName] = useState(initial.name);
    const [type, setType] = useState(initial.type);
    const [options, setOptions] = useState(initial.options.join('\n'));
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save({id: initial.id, name, type, options: type === 'dropdown' || type === 'multiselect' ? options.split('\n') : []}, close);
    };
    return <form className="property-editor" onSubmit={submit}><fieldset disabled={busy}>
        <h3>{initial.id ? 'Edit property' : 'New property'}</h3>
        <div className="form-grid">
            <Label>Property name<Input required autoFocus value={name} onChange={e => setName(e.target.value)}/></Label>
            <Label title={initial.id ? 'Existing property types cannot be changed.' : undefined}>Property type<NativeSelect value={type} disabled={!!initial.id} onChange={e => setType(e.target.value)}>
                <NativeSelectOption value="text">Text</NativeSelectOption><NativeSelectOption value="number">Number</NativeSelectOption>
                <NativeSelectOption value="checkbox">Checkbox</NativeSelectOption><NativeSelectOption value="dropdown">Dropdown</NativeSelectOption>
                <NativeSelectOption value="multiselect">Dropdown (multiple selection)</NativeSelectOption>
            </NativeSelect></Label>
        </div>
        {(type === 'dropdown' || type === 'multiselect') && <Label>Dropdown options (one per line)<Textarea required rows={4} value={options} onChange={e => setOptions(e.target.value)}/></Label>}
        <p className="hint">Existing property types cannot be changed. Dropdown options used by projects must be cleared or changed on those projects before removing the option.</p>
        <div className="dialog-actions"><Button type="button" onClick={close}>Cancel</Button><Button className="primary">Save property</Button></div>
    </fieldset></form>;
}

export default function SettingsView({settings, definitions, busy, saveSettings, saveProperty, moveProperty, removeProperty, previewReminder}: {
    settings: tracker.Settings; definitions: Definition[]; busy: boolean;
    saveSettings: (s: tracker.Settings) => Promise<void>;
    saveProperty: (d: Definition, done: () => void) => Promise<void>;
    moveProperty: (id: string, direction: number) => Promise<void>;
    removeProperty: (d: Definition) => void;
    previewReminder: (running: boolean) => Promise<void>;
}) {
    const [editing, setEditing] = useState<Definition | null>(null);
    return <div className="settings-panels">
        <Card className="panel">
            <div className="section-heading"><div><h2>Appearance</h2><p>Your preference is saved on this computer.</p></div></div>
            <Label className="appearance-control">Color theme<NativeSelect value={settings.theme} disabled={busy} onChange={e => void saveSettings(Object.assign(new tracker.Settings(), settings, {theme: e.target.value}))}>
                <NativeSelectOption value="light">Light</NativeSelectOption><NativeSelectOption value="dark">Dark</NativeSelectOption><NativeSelectOption value="system">Follow Windows / system</NativeSelectOption>
            </NativeSelect></Label>
            <p className="hint">System mode follows the operating system's color preference and updates when it changes.</p>
        </Card>
        <Card className="panel">
            <div className="section-heading"><div><h2>Temporary timer</h2><p>Choose how unassigned time is saved.</p></div></div>
            <Label>Temporary booking behavior<NativeSelect value={settings.requireTemporaryProject ? 'require' : 'allow'} disabled={busy}
                onChange={e => void saveSettings(Object.assign(new tracker.Settings(), settings, {requireTemporaryProject: e.target.value === 'require'}))}>
                <NativeSelectOption value="allow">Allow temporary bookings</NativeSelectOption>
                <NativeSelectOption value="require">Require a project before saving</NativeSelectOption>
            </NativeSelect></Label>
            <p className="hint">Requiring a project prompts before stopping, pausing, switching timers, or quitting. Cancel keeps the temporary timer running. Existing temporary bookings are unchanged.</p>
        </Card>
        <ReminderForm key={JSON.stringify(settings.reminders)} settings={settings} busy={busy} save={saveSettings} preview={previewReminder}/>
        <Card className="panel">
            <div className="section-heading"><div><h2>Project properties</h2><p>Shared optional fields, filled in separately for each project and subproject.</p></div>
                <Button className="primary" disabledReason={whyDisabled([busy, BUSY_REASON], [!!editing, 'Finish or cancel the property you are editing first.'])} onClick={() => setEditing({id: '', name: '', type: 'text', options: []})}>Add property</Button></div>
            {!definitions.length && !editing && <p className="hint">Add fields such as Cost center, Hourly rate, Billable, or Status. Values appear in the project editor and Excel exports.</p>}
            {definitions.length > 0 && <Table className="project-table"><TableHeader><TableRow><TableHead>Property</TableHead><TableHead>Type</TableHead><TableHead>Options</TableHead><TableHead>Actions</TableHead></TableRow></TableHeader>
                <TableBody>{definitions.map((def, index) => <TableRow key={def.id}><TableCell><strong>{def.name}</strong></TableCell><TableCell>{def.type}</TableCell><TableCell>{def.options.join(', ') || '-'}</TableCell>
                    <TableCell className="row-actions"><ButtonGroup aria-label={`Order property ${def.name}`}>
                        <Button disabledReason={whyDisabled([busy, BUSY_REASON], [!!editing, 'Finish or cancel the property you are editing first.'], [index === 0, 'This property is already first.'])} aria-label={`Move property ${def.name} up`} onClick={() => void moveProperty(def.id, -1)}>Move up</Button>
                        <Button disabledReason={whyDisabled([busy, BUSY_REASON], [!!editing, 'Finish or cancel the property you are editing first.'], [index === definitions.length - 1, 'This property is already last.'])} aria-label={`Move property ${def.name} down`} onClick={() => void moveProperty(def.id, 1)}>Move down</Button>
                    </ButtonGroup><Button disabledReason={whyDisabled([busy, BUSY_REASON], [!!editing, 'Finish or cancel the property you are editing first.'])} aria-label={`Edit property ${def.name}`} onClick={() => setEditing(def)}>Edit</Button>
                        <Button disabledReason={whyDisabled([busy, BUSY_REASON], [!!editing, 'Finish or cancel the property you are editing first.'])} className="danger" aria-label={`Delete property ${def.name}`} onClick={() => removeProperty(def)}>Delete</Button></TableCell></TableRow>)}</TableBody>
            </Table>}
            {editing && <PropertyEditor key={editing.id} initial={editing} busy={busy} save={saveProperty} close={() => setEditing(null)}/>}
            <p className="hint">Move properties up or down to set their order in project forms and Excel exports. Changes are saved immediately. Deleting a definition requires confirmation and removes its values from all projects, including archived projects. Excel uses current property names and values, not historical snapshots.</p>
        </Card>
        <Card className="panel">
            <div className="section-heading"><div><h2>About</h2><p>TimeManager version <strong>{__APP_VERSION__}</strong></p></div></div>
        </Card>
    </div>;
}

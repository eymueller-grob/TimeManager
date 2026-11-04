import {BUSY_REASON, Button, Check, MultiSelect, parseMultiselect, ProjectIcon, ProjectMark, RunningSpinner, whyDisabled} from './Controls';
import {cropRect, DEFAULT_ICON_CROP, emojiIcon, ICON_FILE_TYPES, invertIcon, loadIconImage, MAX_ICON_ZOOM, PROJECT_ICON_SIZE, renderIconImage, type IconCrop, type IconImage} from './projectIcon';
import {Trash2} from 'lucide-react';
import {Slider} from './components/ui/slider';
import {Input} from './components/ui/input';
import {Textarea} from './components/ui/textarea';
import {Label} from './components/ui/label';
import {NativeSelect, NativeSelectOption} from './components/ui/native-select';
import {useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode} from 'react';
import {
    DeleteBooking, DeleteProject, ExportBookings, GetState, RestoreProject,
    SaveBooking, SaveProject, SaveProjectWithRelated, StopTimer, PauseTimer, ResumeTimer, ToggleFavorite, ToggleTimer,
    SaveSettings, SavePropertyDefinition, MovePropertyDefinition, DeletePropertyDefinition, TestReminder,
    RunInBackground, QuitApplication,
    UpdateTimerStart,
    StartTemporaryTimer,
    AssignTemporaryTimer,
    DiscardTemporaryTimer,
} from '../wailsjs/go/main/App';
import {main, type tracker} from '../wailsjs/go/models';
import {EventsOn, WindowSetDarkTheme, WindowSetLightTheme} from '../wailsjs/runtime/runtime';
import SettingsView from './SettingsView';
import DatePicker from './DatePicker';
import {isoWeek} from './calendarDate';
import {Dialog, DialogContent, DialogHeader, DialogTitle} from './components/ui/dialog';
import {ButtonGroup} from './components/ui/button-group';
import {ProjectTree, ProjectPicker, projectPath} from './ProjectTree';
import {Card} from './components/ui/card';
import './App.css';

type Project = tracker.Project;
type Booking = main.Booking;
type State = main.State;
type Confirm = {title: string; message: string; action: () => Promise<void>};

const pad = (n: number) => String(n).padStart(2, '0');
const dateKey = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const localInput = (d: Date) => `${dateKey(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
const timeLabel = (d: Date) => d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
const duration = (milliseconds: number) => {
    const seconds = Math.max(0, Math.floor(milliseconds / 1000));
    return `${pad(Math.floor(seconds / 3600))}:${pad(Math.floor(seconds / 60) % 60)}:${pad(seconds % 60)}`;
};
const addDays = (d: Date, days: number) => {
    const result = new Date(d);
    result.setDate(result.getDate() + days);
    return result;
};
const monday = (d: Date) => {
    const result = addDays(d, -((d.getDay() + 6) % 7));
    result.setHours(0, 0, 0, 0);
    return result;
};
const hoursInRange = (b: Booking, start: Date, end: Date) =>
    Math.max(0, Math.min(new Date(b.end).getTime(), end.getTime()) - Math.max(new Date(b.start).getTime(), start.getTime()));

/** Human-readable property value for summaries and search. */
function propertyText(def: tracker.PropertyDefinition, value: string): string {
    if (def.type === 'checkbox') return value === 'true' ? 'Yes' : 'No';
    if (def.type === 'multiselect') return parseMultiselect(value).join(', ');
    return value;
}

function matchesProjectSearch(projects: Project[], project: Project, definitions: tracker.PropertyDefinition[], search: string): boolean {
    const properties = definitions.flatMap(def => {
        const value = project.properties[def.id];
        if (value === undefined) return [];
        return [def.name, value, propertyText(def, value)];
    });
    return `${project.number} ${projectPath(projects, project.id)} ${project.customer} ${properties.join(' ')}`.toLowerCase().includes(search.toLowerCase());
}

function Modal({title, close, busy, children}: {title: string; close: () => void; busy: boolean; children: ReactNode}) {
    const previousFocus = useRef(document.activeElement);
    return <Dialog open onOpenChange={open => {if (!open && !busy) close();}}>
        <DialogContent className="app-dialog" showCloseButton={false} aria-describedby={undefined}
            onEscapeKeyDown={event => {if (busy) event.preventDefault();}}
            onPointerDownOutside={event => event.preventDefault()}
            onCloseAutoFocus={event => {
                if (previousFocus.current instanceof HTMLElement && previousFocus.current.isConnected) {
                    event.preventDefault();
                    previousFocus.current.focus();
                }
            }}>
            <DialogHeader className="dialog-heading"><DialogTitle>{title}</DialogTitle></DialogHeader>
            {children}
        </DialogContent>
    </Dialog>;
}

/** Distinct icons used by projects, in project order, with the names of the projects using each icon. */
function usedIcons(projects: Project[]): {icon: string; names: string[]}[] {
    const byIcon = new Map<string, string[]>();
    for (const p of projects) {
        if (!p.icon) continue;
        const names = byIcon.get(p.icon);
        if (names) names.push(p.name); else byIcon.set(p.icon, [p.name]);
    }
    return [...byIcon].map(([icon, names]) => ({icon, names}));
}

function IconEditor({project, projects, busy, change}: {project: Project; projects: Project[]; busy: boolean; change: (icon: string | undefined) => void}) {
    const used = useMemo(() => usedIcons(projects), [projects]);
    const [symbol, setSymbol] = useState('');
    const [iconError, setIconError] = useState('');
    const [loading, setLoading] = useState(false);
    const [pending, setPending] = useState<IconImage | null>(null);
    const [crop, setCrop] = useState<IconCrop>(DEFAULT_ICON_CROP);
    const fileInput = useRef<HTMLInputElement>(null);
    const fail = (err: unknown) => setIconError(err instanceof Error ? err.message : String(err));
    const attempt = (render: () => string) => {
        setIconError('');
        try {
            change(render());
            return true;
        } catch (err: unknown) {
            fail(err);
            return false;
        }
    };
    const applySymbol = () => { attempt(() => emojiIcon(symbol, project.color)); };
    const upload = async (file: File | undefined) => {
        if (!file) return;
        setIconError('');
        setLoading(true);
        try {
            setPending(await loadIconImage(file));
            setCrop(DEFAULT_ICON_CROP);
        } catch (err: unknown) {
            fail(err);
        } finally {
            setLoading(false);
            if (fileInput.current) fileInput.current.value = '';
        }
    };
    const invert = async () => {
        if (!project.icon) return;
        setIconError('');
        setLoading(true);
        try {
            change(await invertIcon(project.icon));
        } catch (err: unknown) {
            fail(err);
        } finally {
            setLoading(false);
        }
    };
    const finishCrop = (render: () => string) => { if (attempt(render)) setPending(null); };
    const reason = whyDisabled([busy, BUSY_REASON], [loading, 'Please wait until the image is processed.'],
        [!!pending, 'Apply or cancel the crop first.']);
    return <section className="icon-editor" aria-label="Project icon">
        <div className="icon-preview" aria-label={project.icon ? 'Current project icon' : 'No project icon'} role="img"><ProjectMark project={project}/></div>
        <div className="icon-controls">
            <Label>Project icon (emoji or short text)<Input value={symbol} maxLength={8} placeholder="e.g. 🚀 or AB"
                onChange={e => setSymbol(e.target.value)} onKeyDown={e => {if (e.key === 'Enter') {e.preventDefault(); if (!reason) applySymbol();}}}/></Label>
            <ButtonGroup aria-label="Project icon actions">
                <Button type="button" disabledReason={whyDisabled([!!reason, reason ?? ''], [!symbol.trim(), 'Enter an emoji or short text first.'])} onClick={applySymbol}>Use emoji</Button>
                <Button type="button" disabledReason={reason} onClick={() => fileInput.current?.click()}>{loading ? 'Loading image…' : 'Upload image…'}</Button>
                <Button type="button" disabledReason={whyDisabled([!!reason, reason ?? ''], [!project.icon, 'This project has no icon.'])} onClick={() => void invert()}>Invert colors</Button>
                <Button type="button" disabledReason={whyDisabled([!!reason, reason ?? ''], [!project.icon, 'This project has no icon.'])} onClick={() => {setIconError(''); change(undefined);}}>Remove icon</Button>
            </ButtonGroup>
            {used.length > 0 && <div className="used-icons" role="group" aria-label="Already used icons">
                <span className="hint">Already used:</span>
                {used.map(({icon, names}) => {
                    const label = `Use icon of ${names.join(', ')}`;
                    return <Button key={icon} type="button" variant="outline" size="icon" aria-label={label} aria-pressed={project.icon === icon}
                        title={label} disabledReason={reason} onClick={() => {setIconError(''); change(icon);}}>
                        <img src={icon} alt="" aria-hidden="true"/>
                    </Button>;
                })}
            </div>}                        <input ref={fileInput} type="file" hidden aria-label="Project icon image file" accept={`${ICON_FILE_TYPES.join(',')},.svg`}
                onChange={e => void upload(e.target.files?.[0])}/>
            {pending && <IconCropper source={pending} crop={crop} setCrop={setCrop} busy={busy}
                apply={() => finishCrop(() => renderIconImage(pending, crop))}
                whole={() => finishCrop(() => renderIconImage(pending))}
                cancel={() => {setIconError(''); setPending(null);}}/>}
            <p className="hint">Press Win + . to open the Windows emoji picker. Uploaded images (PNG, JPEG, GIF, WebP, or SVG) can be cropped and are scaled to {PROJECT_ICON_SIZE}×{PROJECT_ICON_SIZE} pixels. The icon appears in the app and the system tray.</p>
            {iconError && <p className="error" role="alert">{iconError}</p>}
        </div>
    </section>;
}

function IconCropper({source, crop, setCrop, busy, apply, whole, cancel}: {
    source: IconImage; crop: IconCrop; setCrop: (crop: IconCrop) => void; busy: boolean; apply: () => void; whole: () => void; cancel: () => void;
}) {
    const area = cropRect(source.width, source.height, crop);
    const [preview, previewError] = useMemo(() => {
        try {
            return [renderIconImage(source, crop), ''];
        } catch (err: unknown) {
            return ['', err instanceof Error ? err.message : String(err)];
        }
    }, [source, crop]);
    const fixedX = area.size >= source.width, fixedY = area.size >= source.height;
    const busyReason = busy ? BUSY_REASON : undefined;
    return <section className="icon-cropper" aria-label="Crop image">
        <div className="icon-preview">{preview && <img className="project-icon" src={preview} alt="Cropped icon preview"/>}</div>
        <div className="icon-crop-controls">
            <Label>Zoom ({Math.round(crop.zoom * 100)} %)<Slider min={100} max={MAX_ICON_ZOOM * 100} step={10} value={[Math.round(crop.zoom * 100)]} thumbLabels={['Zoom']}
                onValueChange={([zoom]) => setCrop({...crop, zoom: zoom / 100})}/></Label>
            <Label title={fixedX ? 'The whole width is already used; zoom in to move horizontally.' : undefined}>Horizontal position<Slider disabled={fixedX} min={0} max={100} value={[crop.x]} thumbLabels={['Horizontal position']}
                onValueChange={([x]) => setCrop({...crop, x})}/></Label>
            <Label title={fixedY ? 'The whole height is already used; zoom in to move vertically.' : undefined}>Vertical position<Slider disabled={fixedY} min={0} max={100} value={[crop.y]} thumbLabels={['Vertical position']}
                onValueChange={([y]) => setCrop({...crop, y})}/></Label>
            <ButtonGroup aria-label="Crop actions">
                <Button type="button" className="primary" disabledReason={whyDisabled([busy, BUSY_REASON], [!preview, previewError || 'The preview is not ready.'])} onClick={apply}>Apply crop</Button>
                <Button type="button" disabledReason={busyReason} onClick={whole}>Use whole image</Button>
                <Button type="button" disabledReason={busyReason} onClick={cancel}>Cancel crop</Button>
            </ButtonGroup>
            {previewError && <p className="error" role="alert">{previewError}</p>}
        </div>
    </section>;
}

/** IDs of all other projects in id's tree (top-level ancestor and all its descendants). */
function relatedProjectIds(projects: Project[], id: string): string[] {
    const byId = new Map(projects.map(p => [p.id, p]));
    if (!byId.has(id)) return [];
    let root = id;
    const seen = new Set([root]);
    for (let parent = byId.get(root)?.parentId ?? ''; parent && !seen.has(parent) && byId.has(parent); parent = byId.get(parent)?.parentId ?? '') {
        root = parent;
        seen.add(parent);
    }
    const related: string[] = [];
    const visited = new Set<string>();
    const queue = [root];
    while (queue.length) {
        const current = queue.shift() ?? '';
        if (visited.has(current)) continue;
        visited.add(current);
        if (current !== id) related.push(current);
        queue.push(...projects.filter(p => p.parentId === current).map(p => p.id));
    }
    return related;
}

function ProjectForm({initial, projects, definitions, busy, error, close, save}: {
    initial: Project; projects: Project[]; definitions: tracker.PropertyDefinition[]; busy: boolean; error: string; close: () => void; save: (p: Project, applyToRelated: boolean) => Promise<void>;
}) {
    const [value, setValue] = useState(initial);
    const [applyToRelated, setApplyToRelated] = useState(true);
    const differingRelatives = useMemo(() => {
        const id = value.id || '\u0000new-project';
        const tree = [...projects.filter(p => p.id !== value.id), {...value, id}];
        const related = new Set(relatedProjectIds(tree, id));
        return projects.filter(p => related.has(p.id) && (p.color !== value.color || (p.icon ?? '') !== (value.icon ?? '')));
    }, [projects, value]);
    const isDescendant = (p: Project) => {
        let id = p.id;
        const seen = new Set<string>();
        while (id && !seen.has(id)) {
            if (id === initial.id) return true;
            seen.add(id);
            id = projects.find(p => p.id === id)?.parentId ?? '';
        }
        return false;
    };
    const submit = (e: FormEvent) => { e.preventDefault(); void save(value, applyToRelated && differingRelatives.length > 0); };
    return <Modal title={initial.id ? 'Edit project' : 'Create project'} close={close} busy={busy}>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <div className="form-grid">
                <Label>Project number<Input required autoFocus={!initial.number || !!initial.id} value={value.number} onChange={e => setValue({...value, number: e.target.value})}/></Label>
                <Label>Customer<Input required value={value.customer} onChange={e => setValue({...value, customer: e.target.value})}/></Label>
            </div>
            <Label>Project name<Input required autoFocus={!!initial.number && !initial.id} value={value.name} onChange={e => setValue({...value, name: e.target.value})}/></Label>
            <p className="hint">Project numbers may repeat. Project names must be unique, including archived projects.</p>
            <ProjectPicker label="Parent project" projects={projects} value={value.parentId} emptyLabel="None (top-level project)"
                eligible={projects.filter(p => (!p.archived || p.id === initial.parentId) && !isDescendant(p))}
                onChange={parentId => setValue({...value, parentId})}/>
            <div className="form-grid">
                <Label>Project color<Input type="color" value={value.color} onChange={e => setValue({...value, color: e.target.value})}/></Label>
                <Label className="check" title={value.archived ? 'Archived projects cannot be favorites. Restore the project first.' : undefined}><Check  disabled={value.archived} checked={value.favorite} onCheckedChange={checked => setValue({...value, favorite: checked})}/>Add to favorites</Label>
            </div>
            <IconEditor project={value} projects={projects} busy={busy} change={icon => setValue({...value, icon})}/>
            {differingRelatives.length > 0 && <Label className="check" title={`Related projects with a different color or icon: ${differingRelatives.map(p => p.name).join(', ')}`}>
                <Check checked={applyToRelated} onCheckedChange={setApplyToRelated}/>
                Apply color and icon to all related projects ({differingRelatives.length} {differingRelatives.length === 1 ? 'project differs' : 'projects differ'}: parents and subprojects)
            </Label>}
            {definitions.length > 0 && <section className="custom-properties"><h3>Project properties</h3><p className="hint">Optional values for this project. Shared fields are managed in Settings.</p>
                {definitions.map(def => {
                    const current = value.properties[def.id] ?? '';
                    const change = (next: string) => {
                        const properties = {...value.properties};
                        if (next === '') delete properties[def.id];
                        else properties[def.id] = next;
                        setValue({...value, properties});
                    };
                    if (def.type === 'checkbox') return <Label key={def.id} className="check"><Check checked={current === 'true'} onCheckedChange={checked => change(checked ? 'true' : 'false')}/>{def.name}</Label>;
                    if (def.type === 'multiselect') return <div className="property-field" key={def.id}><span>{def.name}</span>
                        <MultiSelect label={def.name} options={def.options} selected={parseMultiselect(current)} onChange={selected => change(selected.length ? JSON.stringify(selected) : '')}/></div>;
                    return <Label key={def.id}>{def.name}
                        {def.type === 'dropdown' ? <NativeSelect value={current} onChange={e => change(e.target.value)}><NativeSelectOption value="">Not set</NativeSelectOption>{def.options.map(option => <NativeSelectOption key={option} value={option}>{option}</NativeSelectOption>)}</NativeSelect> :
                            <Input type={def.type === 'number' ? 'number' : 'text'} step={def.type === 'number' ? 'any' : undefined} value={current} onChange={e => change(e.target.value)}/>}
                    </Label>;
                })}
            </section>}
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><Button type="button" onClick={close}>Cancel</Button><Button className="primary">Save project</Button></div>
        </fieldset></form>
    </Modal>;
}

function BookingForm({initial, projects, definitions, busy, error, close, save, remove}: {
    initial: Booking; projects: Project[]; definitions: tracker.PropertyDefinition[]; busy: boolean; error: string; close: () => void;
    save: (b: Booking) => Promise<void>; remove: () => void;
}) {
    const [projectId, setProjectId] = useState(initial.projectId);
    const [start, setStart] = useState(localInput(new Date(initial.start)));
    const [end, setEnd] = useState(localInput(new Date(initial.end)));
    const [notes, setNotes] = useState(initial.notes);
    const [exported, setExported] = useState(initial.exported ?? false);
    const selectedProject = projects.find(p => p.id === projectId);
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save({id: initial.id, projectId,
            start: start === localInput(new Date(initial.start)) ? initial.start : new Date(start).toISOString(),
            end: end === localInput(new Date(initial.end)) ? initial.end : new Date(end).toISOString(), notes, exported});
    };
    return <Modal title={initial.id ? 'Edit booking' : 'Add booking'} busy={busy} close={close}>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <ProjectPicker label="Project" required autoFocus projects={projects} value={projectId} onChange={setProjectId}
                eligible={projects.filter(p => !p.archived || p.id === initial.projectId)}/>
            <div className="form-grid">
                <DatePicker label="Start" required dateTime showWeekHint={false} showWeekNumbers={false} value={start} onChange={setStart}/>
                <DatePicker label="End" required dateTime showWeekHint={false} showWeekNumbers={false} value={end} onChange={setEnd}/>
            </div>
            <Label>Notes<Textarea rows={3} value={notes} onChange={e => setNotes(e.target.value)}/></Label>
            <Label className="check"><Check  checked={exported} onCheckedChange={checked => setExported(checked)}/>Exported</Label>
            {definitions.length > 0 && selectedProject && <section className="custom-properties booking-properties" aria-label="Project properties">
                <h3>Project properties</h3><p className="hint">Read-only values of {selectedProject.number} - {selectedProject.name}. Edit them in the project editor.</p>
                {definitions.map(def => {
                    const value = selectedProject.properties[def.id];
                    if (def.type === 'checkbox') return <Label key={def.id} className="check" title="Read-only">
                        <Check checked={value === 'true'} onCheckedChange={() => undefined} disabled aria-readonly="true"/>{def.name}{value === undefined ? ' (not set)' : ''}</Label>;
                    return <Label key={def.id}>{def.name}<Input readOnly tabIndex={-1} title="Read-only" placeholder="Not set" value={value === undefined ? '' : propertyText(def, value)}/></Label>;
                })}
            </section>}
            <p className="hint">Times use your local timezone. Bookings cannot overlap or end in the future.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions">
                {initial.id && <Button type="button" className="danger push-left" onClick={remove}>Delete booking</Button>}
                <Button type="button" onClick={close}>Cancel</Button><Button className="primary">Save booking</Button>
            </div>
        </fieldset></form>
    </Modal>;
}

function TimerStartForm({initial, now, busy, error, close, save}: {
    initial: main.Timer; now: Date; busy: boolean; error: string; close: () => void;
    save: (start: string) => Promise<void>;
}) {
    const original = localInput(new Date(initial.start));
    const [start, setStart] = useState(original);
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save(start === original ? initial.start : new Date(start).toISOString());
    };
    return <Modal title="Edit timer start time" busy={busy} close={close}>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <DatePicker label="Timer start time" required autoFocus dateTime max={localInput(now)} value={start} onChange={setStart}/>
            <p className="hint">Times use your local timezone. Move the start earlier or later, including previous days.
                Future times and overlaps with saved bookings are not allowed. The timer keeps running on the same project.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><Button type="button" onClick={close}>Cancel</Button><Button className="primary">Save start time</Button></div>
        </fieldset></form>
    </Modal>;
}

function App() {
    const [state, setState] = useState<State | null>(null);
    const [view, setView] = useState<'time' | 'projects' | 'settings'>('time');
    const [now, setNow] = useState(new Date());
    const [week, setWeek] = useState(monday(new Date()));
    const [calendarView, setCalendarView] = useState<'details' | 'summary'>('details');
    const [busy, setBusy] = useState(false);
    const busyReason = busy ? BUSY_REASON : undefined;
    const busyRef = useRef(false);
    const [error, setError] = useState('');
    const [notice, setNotice] = useState('');
    const [search, setSearch] = useState('');
    const [favoriteSearch, setFavoriteSearch] = useState('');
    const [showArchived, setShowArchived] = useState(false);
    const [projectEdit, setProjectEdit] = useState<Project | null>(null);
    const [bookingEdit, setBookingEdit] = useState<Booking | null>(null);
    const [timerEdit, setTimerEdit] = useState<main.Timer | null>(null);
    const [confirm, setConfirm] = useState<Confirm | null>(null);
    const [exportOpen, setExportOpen] = useState(false);
    const [closeRequested, setCloseRequested] = useState(false);
    const [temporaryAssignment, setTemporaryAssignment] = useState<main.TemporaryAssignment | null>(null);
    const [assignmentProject, setAssignmentProject] = useState('');
    const [exportFrom, setExportFrom] = useState(dateKey(monday(new Date())));
    const [exportTo, setExportTo] = useState(dateKey(new Date()));
    const [markExported, setMarkExported] = useState(false);
    const [excludeExported, setExcludeExported] = useState(false);
    const requests = useRef(0);
    const errorMessage = (e: unknown) => e instanceof Error ? e.message : String(e);

    const theme = state?.settings.theme;
    useEffect(() => {
        if (!theme) return;
        const media = window.matchMedia('(prefers-color-scheme: dark)');
        const apply = () => {
            const dark = theme === 'dark' || (theme === 'system' && media.matches);
            document.documentElement.dataset.theme = dark ? 'dark' : 'light';
            document.documentElement.classList.toggle('dark', dark);
            if (dark) WindowSetDarkTheme();
            else WindowSetLightTheme();
        };
        apply();
        media.addEventListener('change', apply);
        return () => media.removeEventListener('change', apply);
    }, [theme]);

    const refresh = useCallback(async () => {
        const request = ++requests.current;
        const next = await GetState();
        if (request === requests.current) {
            setNow(new Date());
            setState(next);
        }
    }, []);

    useEffect(() => {
        const load = () => { void refresh().catch((e: unknown) => setError(errorMessage(e))); };
        const off = EventsOn('data-changed', load);
        const offError = EventsOn('app-error', (e: unknown) => setError(errorMessage(e)));
        const offClose = EventsOn('close-requested', () => {setError(''); setCloseRequested(true);});
        const offAssignment = EventsOn('temporary-assignment-requested', (request: main.TemporaryAssignment) => {
            setError('');
            setCloseRequested(false);
            setProjectEdit(null);
            setBookingEdit(null);
            setTimerEdit(null);
            setConfirm(null);
            setExportOpen(false);
            setAssignmentProject('');
            setTemporaryAssignment(request);
        });
        load();
        const tick = setInterval(() => setNow(new Date()), 1000);
        return () => { off(); offError(); offClose(); offAssignment(); clearInterval(tick); requests.current++; };
    }, [refresh]);

    async function run(action: () => Promise<void>, success?: () => void) {
        if (busyRef.current) return;
        busyRef.current = true;
        setBusy(true);
        setError('');
        setNotice('');
        try {
            await action();
            await refresh();
            success?.();
        } catch (e: unknown) {
            setError(errorMessage(e));
        } finally {
            busyRef.current = false;
            setBusy(false);
        }
    }

    function newProject(parent?: Project) {
        setError('');
        setProjectEdit({id: '', number: '', name: '', customer: parent?.customer ?? '',
            parentId: parent?.id ?? '', color: parent?.color ?? '#4f6bed', icon: parent?.icon, favorite: true, archived: false, properties: {}});
    }

    function copyProject(project: Project) {
        setError('');
        setProjectEdit({...project, id: '', name: '', archived: false,
            parentId: state?.projects.some(p => p.id === project.parentId && !p.archived) ? project.parentId : '',
            properties: {...project.properties}});
    }

    function confirmDeleteBooking(booking: main.Booking) {
        const start = new Date(booking.start), end = new Date(booking.end);
        const name = state?.projects.find(p => p.id === booking.projectId)?.name ?? 'Unknown project';
        const range = dateKey(start) === dateKey(end)
            ? `${start.toLocaleDateString()} ${timeLabel(start)} - ${timeLabel(end)}`
            : `${start.toLocaleDateString()} ${timeLabel(start)} - ${end.toLocaleDateString()} ${timeLabel(end)}`;
        setError('');
        setConfirm({title: 'Delete booking?', message: `The booking for ${name} (${range}) will be permanently deleted.`, action: () => DeleteBooking(booking.id)});
    }

    function newBooking(day = new Date()) {
        const start = new Date(day);
        start.setHours(9, 0, 0, 0);
        const end = new Date(day);
        end.setHours(10, 0, 0, 0);
        if (end > now && dateKey(day) === dateKey(now)) {
            end.setTime(now.getTime());
            start.setTime(end.getTime() - 3600000);
        }
        setError('');
        setBookingEdit({id: '', projectId: state?.active?.projectId ?? state?.projects.find(p => !p.archived)?.id ?? '',
            start: start.toISOString(), end: end.toISOString(), notes: '', exported: false});
    }

    if (!state) return <main className="loading"><h1>TimeManager</h1><p>{error ? 'Could not load your saved data.' : 'Loading your workspace...'}</p>
        {error && <p className="error" role="alert">{error}</p>}
        {error && <Button onClick={() => void run(refresh)}>Retry</Button>}
    </main>;

    const projects = state.projects;
    const active = state.active;
    const activeProject = projects.find(p => p.id === active?.projectId);
    const pausedProject = projects.find(p => p.id === state.pausedProjectId);
    const favorites = projects.filter(p => p.favorite && !p.archived);
    const filteredFavorites = favorites.filter(p => matchesProjectSearch(projects, p, state.propertyDefinitions, favoriteSearch.trim()));
    const today = new Date(now); today.setHours(0, 0, 0, 0);
    const bookedToday = state.bookings.reduce((sum, b) => sum + hoursInRange(b, today, addDays(today, 1)), 0);
    const bookedWeek = state.bookings.reduce((sum, b) => sum + hoursInRange(b, week, addDays(week, 7)), 0);
    const ordered = [...projects].sort((a, b) => projectPath(projects, a.id).localeCompare(projectPath(projects, b.id), undefined, {numeric: true}));
    const filtered = ordered.filter(p => (showArchived || !p.archived) && matchesProjectSearch(projects, p, state.propertyDefinitions, search));
    const modalOpen = !!(projectEdit || bookingEdit || timerEdit || confirm || exportOpen || closeRequested || temporaryAssignment);

    return <div className="shell">
        {closeRequested && <Modal title="Close TimeManager?" busy={busy} close={() => setCloseRequested(false)}>
            <p>Keep TimeManager running in the background?</p>
            <p>Run in background hides the window to the Windows tray and keeps timers and reminders running.
                Quit stops and saves any running timer before exiting.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions">
                <Button disabledReason={busyReason} onClick={() => setCloseRequested(false)}>Cancel</Button>
                <Button disabledReason={busyReason} onClick={() => void run(QuitApplication)}>Quit</Button>
                <Button className="primary" disabledReason={busyReason} onClick={() => void run(RunInBackground, () => setCloseRequested(false))}>Run in background</Button>
            </div>
        </Modal>}
        <aside className="sidebar">
            <div className="brand"><span className="brand-mark">T</span><div>TimeManager<small>Make your time count.</small></div></div>
            <nav aria-label="Main navigation">
                <Button className={view === 'time' ? 'selected' : ''} onClick={() => setView('time')}>Time &amp; calendar</Button>
                <Button className={view === 'projects' ? 'selected' : ''} onClick={() => setView('projects')}>Projects</Button>
                <Button className={view === 'settings' ? 'selected' : ''} onClick={() => setView('settings')}>Settings</Button>
            </nav>
            <div className="sidebar-bottom"><ProjectMark project={activeProject ?? pausedProject}/>
                <div>{activeProject ? 'Tracking in progress' : pausedProject ? 'Timer paused' : 'Ready when you are'}<small>Saved locally on this computer</small></div>
            </div>
        </aside>
        <main className="workspace">
            <header className="page-heading"><h1>{view === 'time' ? 'Time & calendar' : view === 'projects' ? 'Projects' : 'Settings'}</h1>
                <Button disabledReason={busyReason} onClick={() => {setError(''); setExportOpen(true);}}>Export to Excel</Button></header>
            {!modalOpen && error && <div className="error banner" role="alert">{error}<Button onClick={() => setError('')}>Dismiss</Button></div>}
            {notice && <div className="success banner" role="status">{notice}<Button onClick={() => setNotice('')}>Dismiss</Button></div>}
            <div className={`timer-dock${active || pausedProject ? ' sticky' : ''}`}><section aria-label="Current timer" className={`timer-panel ${active ? 'running sticky' : pausedProject ? 'paused sticky' : ''}`} style={{borderLeftColor: (activeProject ?? pausedProject)?.color ?? '#cbd5e1'}}>
                <div><p className="eyebrow">{active ? 'CURRENTLY TRACKING' : pausedProject ? 'TIMER PAUSED' : 'NO TIMER RUNNING'}</p>
                    <h2>{active && <RunningSpinner/>}<ProjectIcon project={activeProject ?? pausedProject}/>{activeProject ? `${activeProject.number} - ${projectPath(projects, activeProject.id)}` : pausedProject ? `${pausedProject.number} - ${projectPath(projects, pausedProject.id)}` : 'Start with a favorite project'}</h2>
                    <p>{active ? `Started ${new Date(active.start).toLocaleString()} - ${activeProject?.customer}` : pausedProject ? 'The current segment is saved. Resume to create a new booking segment.' : 'Click once to start. Click again to stop and save.'}</p>
                </div>
                <div className="timer-right"><output className="timer-digits" aria-label="Elapsed time">{duration(active ? now.getTime() - new Date(active.start).getTime() : 0)}</output>
                    {active && <ButtonGroup className="timer-actions" aria-label="Timer actions">
                        <Button disabledReason={busyReason} onClick={() => {setError(''); setTimerEdit(active);}}>Edit start time</Button>
                        {active.projectId === 'timemanager-temporary-project' && <Button disabledReason={busyReason} onClick={() => {
                            setError('');
                            setAssignmentProject('');
                            setTemporaryAssignment(new main.TemporaryAssignment({timer: active, action: 'assign-running', id: ''}));
                        }}>Set project</Button>}
                        <Button disabledReason={busyReason} onClick={() => void run(PauseTimer)}>Pause</Button>
                        <Button className="primary" disabledReason={busyReason} onClick={() => void run(StopTimer)}>Stop &amp; save</Button>
                    </ButtonGroup>}
                    {!active && pausedProject && <ButtonGroup className="timer-actions" aria-label="Timer actions">
                        <Button className="primary" disabledReason={busyReason} onClick={() => void run(ResumeTimer)}>Resume</Button>
                    </ButtonGroup>}
                </div>
            </section></div>
            {pausedProject && <p className="hint">Paused time is not included in bookings. Each resume starts a separate booking segment.</p>}
            {view === 'time' && <p><Button disabledReason={busyReason} onClick={() => void run(StartTemporaryTimer)}>Start temporary timer</Button> <span className="hint">{state.settings.requireTemporaryProject
                ? 'Track unassigned time. Choose a project before stopping, pausing, switching, or quitting; use Set project to assign a running session.'
                : 'Track unassigned time. Stop, switch, or quit to save it immediately; use Set project to assign a running session.'}</span></p>}
            {active && new Date(active.start).getTime() > now.getTime() && <p className="error" role="alert">Your system clock is earlier than the timer start. Correct the clock before stopping.</p>}
            {active && dateKey(new Date(active.start)) !== dateKey(now) && <p className="hint">This timer started on an earlier day. It continues across restarts; edit its booking after stopping if needed.</p>}

            {view === 'settings' ? <SettingsView settings={state.settings} definitions={state.propertyDefinitions} busy={busy}
                saveSettings={s => run(() => SaveSettings(s))}
                previewReminder={running => run(() => TestReminder(running))}
                saveProperty={(def, done) => run(() => SavePropertyDefinition(def), done)}
                moveProperty={(id, direction) => run(() => MovePropertyDefinition(id, direction))}
                removeProperty={def => {
                    setError('');
                    setConfirm({title: 'Delete property?', message: `Delete "${def.name}" and its values from all projects, including archived projects? This cannot be undone.`, action: () => DeletePropertyDefinition(def.id)});
                }}/> : view === 'projects' ? <Card className="panel">
                <div className="section-heading"><div><h2>Project directory</h2><p>{projects.filter(p => !p.archived).length} active projects</p></div><Button className="primary" disabledReason={busyReason} onClick={() => newProject()}>Create project</Button></div>
                <div className="filters"><Input aria-label="Search projects" placeholder="Search number, project, customer, or properties..." value={search} onChange={e => setSearch(e.target.value)}/>
                    <Label className="check"><Check  checked={showArchived} onCheckedChange={checked => setShowArchived(checked)}/>Show archived</Label>
                </div>
                {!filtered.length ? <div className="empty"><h3>No projects here yet</h3><p>Create your first project to start booking time.</p></div> :
                    <ProjectTree label="Project directory hierarchy" projects={ordered} matches={filtered} searching={!!search.trim()}
                        render={(p, context) => <div className={`directory-row ${p.archived ? 'archived' : active?.projectId === p.id ? 'running-project' : state.pausedProjectId === p.id ? 'paused-project' : ''}`}>
                            <Button className="star" aria-label={`${p.favorite ? 'Remove' : 'Add'} favorite ${p.number} ${p.name}`} disabledReason={whyDisabled([busy, BUSY_REASON], [p.archived, 'Archived projects cannot be favorites. Restore the project first.'])} onClick={() => void run(() => ToggleFavorite(p.id))}>{p.favorite ? '\u2605' : '\u2606'}</Button>
                            <div className="project-name"><ProjectMark project={p}/><div><strong>{p.number} - {p.name}</strong>
                                {active?.projectId === p.id && <span className="running-badge"><RunningSpinner/>RUNNING</span>}
                                {!active && state.pausedProjectId === p.id && <span className="pill">PAUSED</span>}
                                <small>{p.parentId ? projectPath(projects, p.id) : 'Top-level project'}{p.archived ? ' / Archived' : ''}{context ? ' / Ancestor context' : ''}</small></div></div>
                            <div>{p.customer}
                                {state.propertyDefinitions.filter(def => p.properties[def.id] !== undefined).map(def =>
                                    <small className="property-summary" key={def.id}>{def.name}: {propertyText(def, p.properties[def.id])}</small>)}
                            </div>
                            <div className="row-actions"><Button disabledReason={busyReason} onClick={() => {setError(''); setProjectEdit(p);}}>Edit</Button>
                                <Button disabledReason={busyReason} aria-label={`Copy project ${p.number} ${p.name}`} onClick={() => copyProject(p)}>Copy</Button>
                                {p.archived ? <Button disabledReason={busyReason} onClick={() => void run(() => RestoreProject(p.id))}>Restore</Button> :
                                    <><Button disabledReason={busyReason} onClick={() => newProject(p)}>Add subproject</Button><Button className="danger" disabledReason={busyReason} onClick={() => {
                                        setError('');
                                        const used = state.bookings.some(b => b.projectId === p.id);
                                        setConfirm({title: used ? 'Archive project?' : 'Delete project?', message: used ? `Archive ${p.number} - ${p.name}? Its bookings will be kept. Move or remove any subprojects first.` : `Permanently delete ${p.number} - ${p.name}? Move or remove any subprojects first.`, action: () => DeleteProject(p.id)});
                                    }}>{state.bookings.some(b => b.projectId === p.id) ? 'Archive' : 'Delete'}</Button></>}
                            </div>
                        </div>}/>}
            </Card> : <>
                <section className="favorites-section">
                    <div className="section-heading"><div><h2>Favorite projects</h2><p>Switching projects automatically saves the previous timer.</p></div><Button onClick={() => setView('projects')}>Manage favorites</Button></div>
                    <div className="filters">
                        <Input type="search" aria-label="Search favorite projects" placeholder="Search number, project, customer, or properties..." value={favoriteSearch} onChange={e => setFavoriteSearch(e.target.value)}/>
                        {favoriteSearch && <Button onClick={() => setFavoriteSearch('')}>Clear favorite search</Button>}
                    </div>
                    {!favorites.length ? <div className="empty panel"><h3>Your favorites go here</h3><p>Create a project or mark one as a favorite in Projects.</p><Button className="primary" disabledReason={busyReason} onClick={() => newProject()}>Create project</Button></div> :
                        !filteredFavorites.length ? <div className="empty panel" role="status"><h3>No favorite projects match your search</h3><p>Try a different number, project name, customer, or property, or clear the search.</p></div> :
                        <ProjectTree label="Favorite project hierarchy" projects={ordered} matches={filteredFavorites} searching={!!favoriteSearch.trim()}
                            render={(p, context) => context ? <div className="tree-context"><ProjectMark project={p}/>{p.number} - {p.name}<small>Ancestor project</small></div> :
                            <Button key={p.id} className={`favorite-card ${active?.projectId === p.id ? 'active' : ''}`} style={{borderLeftColor: p.color}}
                            disabledReason={busyReason} aria-pressed={active?.projectId === p.id} onClick={() => void run(() => ToggleTimer(p.id))} aria-label={`${active?.projectId === p.id ? 'Stop' : state.pausedProjectId === p.id ? 'Resume' : 'Start'} timer for ${p.number} ${p.name}`}>
                            <div className="card-top"><span className="project-number"><ProjectIcon project={p}/>{p.number}</span><span className={active?.projectId === p.id ? 'running-badge' : 'pill'}>{active?.projectId === p.id && <RunningSpinner/>}{active?.projectId === p.id ? 'RUNNING' : state.pausedProjectId === p.id ? 'PAUSED' : 'START'}</span></div>
                            <strong>{p.name}</strong><span className="card-customer">{p.customer}</span>
                            {p.parentId && <small>{projectPath(projects, p.parentId)}</small>}
                        </Button>}/>}
                </section>
                <Card className="panel calendar-panel">
                    <div className="section-heading"><div><h2>Booking calendar</h2><p>Today: {(bookedToday / 3600000).toFixed(2)} h booked &nbsp; / &nbsp; Selected week: {(bookedWeek / 3600000).toFixed(2)} h</p></div>
                        <Button className="primary" disabledReason={whyDisabled([busy, BUSY_REASON], [!projects.some(p => !p.archived), 'Create an active project before adding bookings.'])} onClick={() => newBooking()}>Add booking</Button></div>
                    <div className="calendar-toolbar"><div className="week-controls"><ButtonGroup aria-label="Calendar week navigation"><Button aria-label="Previous week" onClick={() => setWeek(addDays(week, -7))}>&lt;</Button><Button onClick={() => setWeek(monday(now))}>This week</Button><Button aria-label="Next week" onClick={() => setWeek(addDays(week, 7))}>&gt;</Button></ButtonGroup>
                        <DatePicker label="Week of" showWeekHint={false} value={dateKey(week)} onChange={value => {
                            if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
                                const day = new Date(`${value}T12:00:00`);
                                if (!Number.isNaN(day.getTime()) && dateKey(day) === value) setWeek(monday(day));
                            }
                        }}/></div>
                        <Label className="calendar-view">Calendar view<NativeSelect value={calendarView} onChange={e => setCalendarView(e.target.value === 'summary' ? 'summary' : 'details')}>
                            <NativeSelectOption value="details">Booking details</NativeSelectOption><NativeSelectOption value="summary">Daily project totals</NativeSelectOption>
                        </NativeSelect></Label>
                        <span className="calendar-date-range"><span>{isoWeek(week)}</span><span>{week.toLocaleDateString([], {month: 'short', day: 'numeric'})} - {addDays(week, 6).toLocaleDateString([], {month: 'short', day: 'numeric', year: 'numeric'})}</span></span>
                    </div>
                    <div className="calendar-grid">{Array.from({length: 7}, (_, i) => {
                        const day = addDays(week, i), end = addDays(day, 1);
                        const bookings = state.bookings.filter(b => hoursInRange(b, day, end) > 0).sort((a, b) => new Date(a.start).getTime() - new Date(b.start).getTime());
                        const total = bookings.reduce((sum, b) => sum + hoursInRange(b, day, end), 0);
                        const projectTotals = new Map<string, number>();
                        for (const booking of bookings) {
                            projectTotals.set(booking.projectId, (projectTotals.get(booking.projectId) ?? 0) + hoursInRange(booking, day, end));
                        }
                        const isToday = dateKey(day) === dateKey(now);
                        const runningHere = active && new Date(active.start) < end && now >= day;
                        return <section key={dateKey(day)} className={`calendar-day ${isToday ? 'today' : ''}`} aria-label={dateKey(day)}>
                            <header><span>{day.toLocaleDateString([], {weekday: 'short'})}</span><strong>{day.getDate()}</strong><small>{(total / 3600000).toFixed(2)} h</small></header>
                            <div className="day-bookings">
                                {calendarView === 'summary' ? (bookings.length > 0 && <ul className="project-totals" aria-label={`Project totals ${dateKey(day)}`}>
                                    {ordered.filter(p => projectTotals.has(p.id)).map(p => {
                                        const allExported = bookings.filter(b => b.projectId === p.id).every(b => b.exported);
                                        return <li key={p.id} className={`booking-card${allExported ? ' exported-booking' : ''}`} style={{borderLeftColor: allExported ? undefined : p.color}}>
                                        <strong><ProjectIcon project={p}/>{p.number} - {p.name}</strong>
                                        <span className="summary-duration">{((projectTotals.get(p.id) ?? 0) / 3600000).toFixed(2)} h</span>
                                        {p.archived && <small>Archived</small>}
                                        {allExported && <small>Exported</small>}
                                    </li>;})}
                                </ul>) : bookings.map(b => {
                                    const p = projects.find(p => p.id === b.projectId);
                                    const start = new Date(Math.max(new Date(b.start).getTime(), day.getTime()));
                                    const finish = new Date(Math.min(new Date(b.end).getTime(), end.getTime()));
                                    const label = `${timeLabel(start)} - ${finish.getTime() === end.getTime() ? '24:00' : timeLabel(finish)}`;
                                    return <div key={b.id} className="booking-entry">
                                        <Button className={`booking-card${b.exported ? ' exported-booking' : ''}`} style={{borderLeftColor: b.exported ? undefined : p?.color}} disabledReason={busyReason} onClick={() => {setError(''); setBookingEdit(b);}}>
                                            <span className="booking-time">{label}</span>
                                            <strong><ProjectIcon project={p}/>{p?.number} - {p?.name}</strong><small>{duration(hoursInRange(b, day, end))}{p?.archived ? ' / Archived' : ''}</small>
                                            {b.notes && <span className="booking-notes">{b.notes}</span>}
                                            {b.exported && <small>Exported</small>}
                                        </Button>
                                        <Button className="booking-delete" variant="ghost" size="icon-xs" aria-label={`Delete booking ${label} ${p?.name ?? ''}`.trim()}
                                            title="Delete booking" disabledReason={busyReason} onClick={() => confirmDeleteBooking(b)}>
                                            <Trash2 aria-hidden="true"/>
                                        </Button>
                                    </div>;
                                })}
                                {calendarView === 'details' && runningHere && <div className="booking-card pending" style={{borderLeftColor: activeProject?.color}}><span>Running since {timeLabel(new Date(Math.max(new Date(active.start).getTime(), day.getTime())))}</span><strong><ProjectIcon project={activeProject}/>{activeProject?.name}</strong><small>Saved when stopped</small></div>}
                                {!bookings.length && (calendarView === 'summary' || !runningHere) && <p className="day-empty">No bookings</p>}
                            </div>
                            <Button className="add-day" aria-label={`Add booking ${dateKey(day)}`} disabledReason={whyDisabled([busy, BUSY_REASON], [day > now, 'Bookings cannot be added for future days.'], [!projects.some(p => !p.archived), 'Create an active project before adding bookings.'])} onClick={() => newBooking(day)}>+ Add</Button>
                        </section>;
                    })}</div>
                    <p className="hint">{calendarView === 'summary' ? 'Daily project totals combine saved bookings per project in decimal hours. Switch to Booking details to edit individual bookings. Projects and subprojects are totaled separately.' : 'Click a booking to edit it, or click its trash can to delete it.'} Overnight bookings appear on each day they cover. Totals exclude the running timer.</p>
                </Card>
            </>}
        </main>

        {temporaryAssignment && <Modal title="Assign temporary time" busy={busy} close={() => setTemporaryAssignment(null)}>
            <form onSubmit={e => {e.preventDefault(); void run(() => AssignTemporaryTimer(temporaryAssignment, assignmentProject), () => setTemporaryAssignment(null));}}>
                <fieldset disabled={busy}>
                    <p>{temporaryAssignment.action === 'assign-running'
                        ? 'Choose a project for this entire running session. The timer keeps running with its original start time; no booking is created. Cancel leaves it temporary.'
                        : 'Choose the project for this temporary time before stopping, pausing, switching, or quitting. The timer keeps running until you confirm. Cancel keeps tracking.'}</p>
                    <ProjectPicker label="Assign to project" required autoFocus projects={ordered} value={assignmentProject} onChange={setAssignmentProject}
                        eligible={ordered.filter(p => !p.archived && p.id !== temporaryAssignment.timer.projectId)}/>
                    {!ordered.some(p => !p.archived && p.id !== temporaryAssignment.timer.projectId) && <p className="hint">No project is available. Cancel and create a project first.</p>}
                    <p className="hint">Cancel booking stops and discards this temporary session without saving. It does not switch projects or quit the app. Existing bookings are kept.</p>
                    {error && <p className="error" role="alert">{error}</p>}
                    <div className="dialog-actions">
                        <Button type="button" className="danger push-left" onClick={() => void run(() => DiscardTemporaryTimer(temporaryAssignment.timer), () => setTemporaryAssignment(null))}>Cancel booking</Button>
                        <Button type="button" onClick={() => setTemporaryAssignment(null)}>Cancel</Button><Button className="primary" disabledReason={assignmentProject ? undefined : 'Select a project first.'}>Assign &amp; continue</Button>
                    </div>
                </fieldset>
            </form>
        </Modal>}
        {timerEdit && <TimerStartForm initial={timerEdit} now={now} busy={busy} error={closeRequested ? '' : error} close={() => setTimerEdit(null)}
            save={start => run(() => UpdateTimerStart(timerEdit, start), () => setTimerEdit(null))}/>}
        {projectEdit && <ProjectForm initial={projectEdit} projects={ordered} definitions={state.propertyDefinitions} busy={busy} error={closeRequested ? '' : error} close={() => setProjectEdit(null)}
            save={(p, applyToRelated) => run(() => applyToRelated ? SaveProjectWithRelated(p) : SaveProject(p), () => setProjectEdit(null))}/>}
        {bookingEdit && <BookingForm initial={bookingEdit} projects={ordered} definitions={state.propertyDefinitions} busy={busy} error={closeRequested ? '' : error} close={() => setBookingEdit(null)}
            save={b => run(() => SaveBooking(b), () => setBookingEdit(null))}
            remove={() => {confirmDeleteBooking(bookingEdit); setBookingEdit(null);}}/>}
        {confirm && <Modal title={confirm.title} close={() => setConfirm(null)} busy={busy}>
            <p>{confirm.message}</p>{!closeRequested && error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><Button disabledReason={busyReason} onClick={() => setConfirm(null)}>Cancel</Button><Button disabledReason={busyReason} className="danger" onClick={() => void run(confirm.action, () => setConfirm(null))}>Confirm</Button></div>
        </Modal>}
        {exportOpen && <Modal title="Export bookings to Excel" close={() => setExportOpen(false)} busy={busy}>
            <form onSubmit={e => {e.preventDefault(); void run(async () => {
                const path = await ExportBookings(exportFrom, exportTo, {markExported, excludeExported});
                if (path) {setExportOpen(false); setNotice(`Excel export saved: ${path}`);}
            });}}><fieldset disabled={busy}>
                <div className="form-grid"><DatePicker label="From date" required value={exportFrom} onChange={setExportFrom}/>
                    <DatePicker label="Through date (inclusive)" required min={exportFrom} value={exportTo} onChange={setExportTo}/></div>
                <p className="hint">Exports saved bookings, including archived projects, with customer, project number, hierarchy, local start/end, hours, notes, color, and custom project properties. Bookings crossing the date boundaries are clipped to the selected range. The running timer is not included.</p>
                <Label className="check"><Check  checked={excludeExported} onCheckedChange={checked => setExcludeExported(checked)}/>Exclude already exported bookings</Label>
                <Label className="check"><Check  checked={markExported} onCheckedChange={checked => setMarkExported(checked)}/>Mark fully included bookings as exported after saving</Label>
                <p className="hint">Partially included bookings stay unchanged. Canceling or failing to save the workbook does not mark bookings.</p>
                {!closeRequested && error && <p className="error" role="alert">{error}</p>}
                <div className="dialog-actions"><Button type="button" onClick={() => setExportOpen(false)}>Cancel</Button><Button className="primary">Choose file &amp; export</Button></div>
            </fieldset></form>
        </Modal>}
    </div>;
}

export default App;

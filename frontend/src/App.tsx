import {useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode} from 'react';
import {
    DeleteBooking, DeleteProject, ExportBookings, GetState, RestoreProject,
    SaveBooking, SaveProject, StopTimer, ToggleFavorite, ToggleTimer,
    SaveSettings, SavePropertyDefinition, DeletePropertyDefinition, TestReminder,
    RunInBackground, QuitApplication,
    UpdateTimerStart,
    StartTemporaryTimer,
    AssignTemporaryTimer,
} from '../wailsjs/go/main/App';
import type {main, tracker} from '../wailsjs/go/models';
import {EventsOn, WindowSetDarkTheme, WindowSetLightTheme} from '../wailsjs/runtime/runtime';
import SettingsView from './SettingsView';
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

function projectPath(projects: Project[], id: string): string {
    const names: string[] = [];
    const seen = new Set<string>();
    while (id && !seen.has(id)) {
        seen.add(id);
        const p = projects.find(p => p.id === id);
        if (!p) break;
        names.unshift(p.name);
        id = p.parentId;
    }
    return names.join(' / ');
}

function matchesProjectSearch(projects: Project[], project: Project, search: string): boolean {
    return `${project.number} ${projectPath(projects, project.id)} ${project.customer}`.toLowerCase().includes(search.toLowerCase());
}

function Modal({title, close, busy, children}: {title: string; close: () => void; busy: boolean; children: ReactNode}) {
    const ref = useRef<HTMLDialogElement>(null);
    useEffect(() => {
        const dialog = ref.current;
        dialog?.showModal();
        return () => dialog?.close();
    }, []);
    return <dialog ref={ref} aria-label={title} onCancel={e => { e.preventDefault(); if (!busy) close(); }}>
        <div className="dialog-heading"><h2>{title}</h2><button aria-label="Close dialog" disabled={busy} onClick={close}>Close</button></div>
        {children}
    </dialog>;
}

function ProjectForm({initial, projects, definitions, busy, error, close, save}: {
    initial: Project; projects: Project[]; definitions: tracker.PropertyDefinition[]; busy: boolean; error: string; close: () => void; save: (p: Project) => Promise<void>;
}) {
    const [value, setValue] = useState(initial);
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
    const submit = (e: FormEvent) => { e.preventDefault(); void save(value); };
    return <Modal title={initial.id ? 'Edit project' : 'Create project'} close={close} busy={busy}>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <div className="form-grid">
                <label>Project number<input required autoFocus={!initial.number || !!initial.id} value={value.number} onChange={e => setValue({...value, number: e.target.value})}/></label>
                <label>Customer<input required value={value.customer} onChange={e => setValue({...value, customer: e.target.value})}/></label>
            </div>
            <label>Project name<input required autoFocus={!!initial.number && !initial.id} value={value.name} onChange={e => setValue({...value, name: e.target.value})}/></label>
            <p className="hint">Project numbers may repeat. Project names must be unique, including archived projects.</p>
            <label>Parent project<select value={value.parentId} onChange={e => setValue({...value, parentId: e.target.value})}>
                <option value="">None (top-level project)</option>
                {projects.filter(p => (!p.archived || p.id === initial.parentId) && !isDescendant(p)).map(p =>
                    <option key={p.id} value={p.id}>{p.number} - {projectPath(projects, p.id)}</option>)}
            </select></label>
            <div className="form-grid">
                <label>Project color<input type="color" value={value.color} onChange={e => setValue({...value, color: e.target.value})}/></label>
                <label className="check"><input type="checkbox" disabled={value.archived} checked={value.favorite} onChange={e => setValue({...value, favorite: e.target.checked})}/>Add to favorites</label>
            </div>
            {definitions.length > 0 && <section className="custom-properties"><h3>Project properties</h3><p className="hint">Optional values for this project. Shared fields are managed in Settings.</p>
                {definitions.map(def => {
                    const current = value.properties[def.id] ?? '';
                    const change = (next: string) => {
                        const properties = {...value.properties};
                        if (next === '') delete properties[def.id];
                        else properties[def.id] = next;
                        setValue({...value, properties});
                    };
                    return <label key={def.id}>{def.name}
                        {def.type === 'dropdown' ? <select value={current} onChange={e => change(e.target.value)}><option value="">Not set</option>{def.options.map(option => <option key={option} value={option}>{option}</option>)}</select> :
                            def.type === 'checkbox' ? <select value={current} onChange={e => change(e.target.value)}><option value="">Not set</option><option value="true">Yes</option><option value="false">No</option></select> :
                                <input type={def.type === 'number' ? 'number' : 'text'} step={def.type === 'number' ? 'any' : undefined} value={current} onChange={e => change(e.target.value)}/>}
                    </label>;
                })}
            </section>}
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><button type="button" onClick={close}>Cancel</button><button className="primary">Save project</button></div>
        </fieldset></form>
    </Modal>;
}

function BookingForm({initial, projects, busy, error, close, save, remove}: {
    initial: Booking; projects: Project[]; busy: boolean; error: string; close: () => void;
    save: (b: Booking) => Promise<void>; remove: () => void;
}) {
    const [projectId, setProjectId] = useState(initial.projectId);
    const [start, setStart] = useState(localInput(new Date(initial.start)));
    const [end, setEnd] = useState(localInput(new Date(initial.end)));
    const [notes, setNotes] = useState(initial.notes);
    const submit = (e: FormEvent) => {
        e.preventDefault();
        void save({id: initial.id, projectId,
            start: start === localInput(new Date(initial.start)) ? initial.start : new Date(start).toISOString(),
            end: end === localInput(new Date(initial.end)) ? initial.end : new Date(end).toISOString(), notes});
    };
    return <Modal title={initial.id ? 'Edit booking' : 'Add booking'} busy={busy} close={close}>
        <form onSubmit={submit}><fieldset disabled={busy}>
            <label>Project<select required autoFocus value={projectId} onChange={e => setProjectId(e.target.value)}>
                <option value="">Select a project</option>
                {projects.filter(p => !p.archived || p.id === initial.projectId).map(p =>
                    <option key={p.id} value={p.id}>{p.number} - {projectPath(projects, p.id)}{p.archived ? ' (archived)' : ''}</option>)}
            </select></label>
            <div className="form-grid">
                <label>Start<input required type="datetime-local" step="1" value={start} onChange={e => setStart(e.target.value)}/></label>
                <label>End<input required type="datetime-local" step="1" value={end} onChange={e => setEnd(e.target.value)}/></label>
            </div>
            <label>Notes<textarea rows={3} value={notes} onChange={e => setNotes(e.target.value)}/></label>
            <p className="hint">Times use your local timezone. Bookings cannot overlap or end in the future.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions">
                {initial.id && <button type="button" className="danger push-left" onClick={remove}>Delete booking</button>}
                <button type="button" onClick={close}>Cancel</button><button className="primary">Save booking</button>
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
            <label>Timer start time<input required autoFocus type="datetime-local" step="1" max={localInput(now)} value={start} onChange={e => setStart(e.target.value)}/></label>
            <p className="hint">Times use your local timezone. Move the start earlier or later, including previous days.
                Future times and overlaps with saved bookings are not allowed. The timer keeps running on the same project.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><button type="button" onClick={close}>Cancel</button><button className="primary">Save start time</button></div>
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
    const requests = useRef(0);
    const errorMessage = (e: unknown) => e instanceof Error ? e.message : String(e);

    const theme = state?.settings.theme;
    useEffect(() => {
        if (!theme) return;
        const media = window.matchMedia('(prefers-color-scheme: dark)');
        const apply = () => {
            const dark = theme === 'dark' || (theme === 'system' && media.matches);
            document.documentElement.dataset.theme = dark ? 'dark' : 'light';
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
            parentId: parent?.id ?? '', color: parent?.color ?? '#4f6bed', favorite: true, archived: false, properties: {}});
    }

    function copyProject(project: Project) {
        setError('');
        setProjectEdit({...project, id: '', name: '', archived: false,
            parentId: state?.projects.some(p => p.id === project.parentId && !p.archived) ? project.parentId : '',
            properties: {...project.properties}});
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
            start: start.toISOString(), end: end.toISOString(), notes: ''});
    }

    if (!state) return <main className="loading"><h1>TimeManager</h1><p>{error ? 'Could not load your saved data.' : 'Loading your workspace...'}</p>
        {error && <p className="error" role="alert">{error}</p>}
        {error && <button onClick={() => void run(refresh)}>Retry</button>}
    </main>;

    const projects = state.projects;
    const active = state.active;
    const activeProject = projects.find(p => p.id === active?.projectId);
    const favorites = projects.filter(p => p.favorite && !p.archived);
    const filteredFavorites = favorites.filter(p => matchesProjectSearch(projects, p, favoriteSearch.trim()));
    const today = new Date(now); today.setHours(0, 0, 0, 0);
    const bookedToday = state.bookings.reduce((sum, b) => sum + hoursInRange(b, today, addDays(today, 1)), 0);
    const bookedWeek = state.bookings.reduce((sum, b) => sum + hoursInRange(b, week, addDays(week, 7)), 0);
    const ordered = [...projects].sort((a, b) => projectPath(projects, a.id).localeCompare(projectPath(projects, b.id), undefined, {numeric: true}));
    const filtered = ordered.filter(p => (showArchived || !p.archived) && matchesProjectSearch(projects, p, search));
    const modalOpen = !!(projectEdit || bookingEdit || timerEdit || confirm || exportOpen || closeRequested || temporaryAssignment);

    return <div className="shell">
        {closeRequested && <Modal title="Close TimeManager?" busy={busy} close={() => setCloseRequested(false)}>
            <p>Keep TimeManager running in the background?</p>
            <p>Run in background hides the window to the Windows tray and keeps timers and reminders running.
                Quit stops and saves any running timer before exiting.</p>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions">
                <button disabled={busy} onClick={() => setCloseRequested(false)}>Cancel</button>
                <button disabled={busy} onClick={() => void run(QuitApplication)}>Quit</button>
                <button className="primary" disabled={busy} onClick={() => void run(RunInBackground, () => setCloseRequested(false))}>Run in background</button>
            </div>
        </Modal>}
        <aside className="sidebar">
            <div className="brand"><span className="brand-mark">T</span><div>TimeManager<small>Make your time count.</small></div></div>
            <nav aria-label="Main navigation">
                <button className={view === 'time' ? 'selected' : ''} onClick={() => setView('time')}>Time &amp; calendar</button>
                <button className={view === 'projects' ? 'selected' : ''} onClick={() => setView('projects')}>Projects</button>
                <button className={view === 'settings' ? 'selected' : ''} onClick={() => setView('settings')}>Settings</button>
            </nav>
            <div className="sidebar-bottom"><span className="dot" style={{background: activeProject?.color ?? '#94a3b8'}}/>
                <div>{activeProject ? 'Tracking in progress' : 'Ready when you are'}<small>Saved locally on this computer</small></div>
            </div>
        </aside>
        <main className="workspace">
            <header className="page-heading"><div><p className="eyebrow">YOUR WORKSPACE</p>
                <h1>{view === 'time' ? 'Time & calendar' : view === 'projects' ? 'Projects' : 'Settings'}</h1>
                <p className="subtitle">{view === 'time' ? 'Pick a project. Start focusing. Your time is saved.' : view === 'projects' ? 'Organize your customers, projects, and subprojects.' : 'Personalize your workspace and project fields.'}</p>
            </div><button disabled={busy} onClick={() => {setError(''); setExportOpen(true);}}>Export to Excel</button></header>
            {!modalOpen && error && <div className="error banner" role="alert">{error}<button onClick={() => setError('')}>Dismiss</button></div>}
            {notice && <div className="success banner" role="status">{notice}</div>}
            <section className={`timer-panel ${active ? 'running' : ''}`} style={{borderLeftColor: activeProject?.color ?? '#cbd5e1'}}>
                <div><p className="eyebrow">{active ? 'CURRENTLY TRACKING' : 'NO TIMER RUNNING'}</p>
                    <h2>{activeProject ? `${activeProject.number} - ${projectPath(projects, activeProject.id)}` : 'Start with a favorite project'}</h2>
                    <p>{active ? `Started ${new Date(active.start).toLocaleString()} - ${activeProject?.customer}` : 'Click once to start. Click again to stop and save.'}</p>
                </div>
                <div className="timer-right"><output className="timer-digits" aria-label="Elapsed time">{duration(active ? now.getTime() - new Date(active.start).getTime() : 0)}</output>
                    {active && <button disabled={busy} onClick={() => {setError(''); setTimerEdit(active);}}>Edit start time</button>}
                    {active && <button className="primary" disabled={busy} onClick={() => void run(StopTimer)}>Stop &amp; save</button>}
                </div>
            </section>
            {view === 'time' && <p><button disabled={busy} onClick={() => void run(StartTemporaryTimer)}>Start temporary timer</button> <span className="hint">Track unassigned time. Choose its project when stopping, switching, or quitting.</span></p>}
            {active && new Date(active.start).getTime() > now.getTime() && <p className="error" role="alert">Your system clock is earlier than the timer start. Correct the clock before stopping.</p>}
            {active && dateKey(new Date(active.start)) !== dateKey(now) && <p className="hint">This timer started on an earlier day. It continues across restarts; edit its booking after stopping if needed.</p>}

            {view === 'settings' ? <SettingsView settings={state.settings} definitions={state.propertyDefinitions} busy={busy}
                saveSettings={s => run(() => SaveSettings(s))}
                previewReminder={running => run(() => TestReminder(running))}
                saveProperty={(def, done) => run(() => SavePropertyDefinition(def), done)}
                removeProperty={def => {
                    setError('');
                    setConfirm({title: 'Delete property?', message: `Delete "${def.name}" and its values from all projects, including archived projects? This cannot be undone.`, action: () => DeletePropertyDefinition(def.id)});
                }}/> : view === 'projects' ? <section className="panel">
                <div className="section-heading"><div><h2>Project directory</h2><p>{projects.filter(p => !p.archived).length} active projects</p></div><button className="primary" disabled={busy} onClick={() => newProject()}>Create project</button></div>
                <div className="filters"><input aria-label="Search projects" placeholder="Search number, project, or customer..." value={search} onChange={e => setSearch(e.target.value)}/>
                    <label className="check"><input type="checkbox" checked={showArchived} onChange={e => setShowArchived(e.target.checked)}/>Show archived</label>
                </div>
                {!filtered.length ? <div className="empty"><h3>No projects here yet</h3><p>Create your first project to start booking time.</p></div> :
                    <div className="table-scroll"><table className="project-table"><thead><tr><th>Favorite</th><th>Project</th><th>Customer</th><th>Actions</th></tr></thead>
                        <tbody>{filtered.map(p => <tr key={p.id} className={p.archived ? 'archived' : active?.projectId === p.id ? 'running-project' : ''}>
                            <td><button className="star" aria-label={`${p.favorite ? 'Remove' : 'Add'} favorite ${p.number} ${p.name}`} disabled={busy || p.archived} onClick={() => void run(() => ToggleFavorite(p.id))}>{p.favorite ? '\u2605' : '\u2606'}</button></td>
                            <td><div className="project-name"><span className="dot" style={{background: p.color}}/><div><strong>{p.number} - {p.name}</strong>
                                {active?.projectId === p.id && <span className="running-badge">RUNNING</span>}
                                <small>{p.parentId ? projectPath(projects, p.id) : 'Top-level project'}{p.archived ? ' / Archived' : ''}</small></div></div></td>
                            <td>{p.customer}
                                {state.propertyDefinitions.filter(def => p.properties[def.id] !== undefined).map(def =>
                                    <small className="property-summary" key={def.id}>{def.name}: {def.type === 'checkbox' ? (p.properties[def.id] === 'true' ? 'Yes' : 'No') : p.properties[def.id]}</small>)}
                            </td>
                            <td className="row-actions"><button disabled={busy} onClick={() => {setError(''); setProjectEdit(p);}}>Edit</button>
                                <button disabled={busy} aria-label={`Copy project ${p.number} ${p.name}`} onClick={() => copyProject(p)}>Copy</button>
                                {p.archived ? <button disabled={busy} onClick={() => void run(() => RestoreProject(p.id))}>Restore</button> :
                                    <><button disabled={busy} onClick={() => newProject(p)}>Add subproject</button><button className="danger" disabled={busy} onClick={() => {
                                        setError('');
                                        const used = state.bookings.some(b => b.projectId === p.id);
                                        setConfirm({title: used ? 'Archive project?' : 'Delete project?', message: used ? `Archive ${p.number} - ${p.name}? Its bookings will be kept. Move or remove any subprojects first.` : `Permanently delete ${p.number} - ${p.name}? Move or remove any subprojects first.`, action: () => DeleteProject(p.id)});
                                    }}>{state.bookings.some(b => b.projectId === p.id) ? 'Archive' : 'Delete'}</button></>}
                            </td>
                        </tr>)}</tbody>
                    </table></div>}
            </section> : <>
                <section className="favorites-section">
                    <div className="section-heading"><div><h2>Favorite projects</h2><p>Switching projects automatically saves the previous timer.</p></div><button onClick={() => setView('projects')}>Manage favorites</button></div>
                    <div className="filters">
                        <input type="search" aria-label="Search favorite projects" placeholder="Search number, project, or customer..." value={favoriteSearch} onChange={e => setFavoriteSearch(e.target.value)}/>
                        {favoriteSearch && <button onClick={() => setFavoriteSearch('')}>Clear favorite search</button>}
                    </div>
                    {!favorites.length ? <div className="empty panel"><h3>Your favorites go here</h3><p>Create a project or mark one as a favorite in Projects.</p><button className="primary" disabled={busy} onClick={() => newProject()}>Create project</button></div> :
                        !filteredFavorites.length ? <div className="empty panel" role="status"><h3>No favorite projects match your search</h3><p>Try a different number, project name, or customer, or clear the search.</p></div> :
                        <div className="favorite-grid">{filteredFavorites.map(p => <button key={p.id} className={`favorite-card ${active?.projectId === p.id ? 'active' : ''}`} style={{borderTopColor: p.color}}
                            disabled={busy} aria-pressed={active?.projectId === p.id} onClick={() => void run(() => ToggleTimer(p.id))} aria-label={`${active?.projectId === p.id ? 'Stop' : 'Start'} timer for ${p.number} ${p.name}`}>
                            <div className="card-top"><span className="project-number">{p.number}</span><span className={active?.projectId === p.id ? 'running-badge' : 'pill'}>{active?.projectId === p.id ? 'RUNNING' : 'START'}</span></div>
                            <strong>{p.name}</strong><span className="card-customer">{p.customer}</span>
                            {p.parentId && <small>{projectPath(projects, p.parentId)}</small>}
                        </button>)}</div>}
                </section>
                <section className="panel calendar-panel">
                    <div className="section-heading"><div><h2>Booking calendar</h2><p>Today: {(bookedToday / 3600000).toFixed(2)} h booked &nbsp; / &nbsp; Selected week: {(bookedWeek / 3600000).toFixed(2)} h</p></div>
                        <button className="primary" disabled={busy || !projects.some(p => !p.archived)} onClick={() => newBooking()}>Add booking</button></div>
                    <div className="calendar-toolbar"><div className="week-controls"><button aria-label="Previous week" onClick={() => setWeek(addDays(week, -7))}>&lt;</button><button onClick={() => setWeek(monday(now))}>This week</button><button aria-label="Next week" onClick={() => setWeek(addDays(week, 7))}>&gt;</button>
                        <label>Week of <input type="date" value={dateKey(week)} onChange={e => { if (e.target.value) setWeek(monday(new Date(`${e.target.value}T12:00:00`))); }}/></label></div>
                        <label className="calendar-view">Calendar view<select value={calendarView} onChange={e => setCalendarView(e.target.value === 'summary' ? 'summary' : 'details')}>
                            <option value="details">Booking details</option><option value="summary">Daily project totals</option>
                        </select></label>
                        <span>{week.toLocaleDateString([], {month: 'short', day: 'numeric'})} - {addDays(week, 6).toLocaleDateString([], {month: 'short', day: 'numeric', year: 'numeric'})}</span>
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
                                    {ordered.filter(p => projectTotals.has(p.id)).map(p => <li key={p.id} className="booking-card" style={{borderLeftColor: p.color}}>
                                        <strong>{p.number} - {p.name}</strong>
                                        <span className="summary-duration">{duration(projectTotals.get(p.id) ?? 0)}</span>
                                        {p.archived && <small>Archived</small>}
                                    </li>)}
                                </ul>) : bookings.map(b => {
                                    const p = projects.find(p => p.id === b.projectId);
                                    const start = new Date(Math.max(new Date(b.start).getTime(), day.getTime()));
                                    const finish = new Date(Math.min(new Date(b.end).getTime(), end.getTime()));
                                    return <button key={b.id} className="booking-card" style={{borderLeftColor: p?.color}} disabled={busy} onClick={() => {setError(''); setBookingEdit(b);}}>
                                        <span className="booking-time">{timeLabel(start)} - {finish.getTime() === end.getTime() ? '24:00' : timeLabel(finish)}</span>
                                        <strong>{p?.number} - {p?.name}</strong><small>{duration(hoursInRange(b, day, end))}{p?.archived ? ' / Archived' : ''}</small>
                                        {b.notes && <span className="booking-notes">{b.notes}</span>}
                                    </button>;
                                })}
                                {calendarView === 'details' && runningHere && <div className="booking-card pending" style={{borderLeftColor: activeProject?.color}}><span>Running since {timeLabel(new Date(Math.max(new Date(active.start).getTime(), day.getTime())))}</span><strong>{activeProject?.name}</strong><small>Saved when stopped</small></div>}
                                {!bookings.length && (calendarView === 'summary' || !runningHere) && <p className="day-empty">No bookings</p>}
                            </div>
                            <button className="add-day" aria-label={`Add booking ${dateKey(day)}`} disabled={busy || day > now || !projects.some(p => !p.archived)} onClick={() => newBooking(day)}>+ Add</button>
                        </section>;
                    })}</div>
                    <p className="hint">{calendarView === 'summary' ? 'Daily project totals combine saved bookings per project in hours:minutes:seconds. Switch to Booking details to edit individual bookings. Projects and subprojects are totaled separately.' : 'Click a booking to edit it.'} Overnight bookings appear on each day they cover. Totals exclude the running timer.</p>
                </section>
            </>}
        </main>

        {temporaryAssignment && <Modal title="Assign temporary time" busy={busy} close={() => setTemporaryAssignment(null)}>
            <form onSubmit={e => {e.preventDefault(); void run(() => AssignTemporaryTimer(temporaryAssignment, assignmentProject), () => setTemporaryAssignment(null));}}>
                <fieldset disabled={busy}>
                    <p>Choose the project for this temporary time before stopping, switching, or quitting. The timer keeps running until you confirm. Cancel keeps tracking.</p>
                    <label>Assign to project<select required autoFocus value={assignmentProject} onChange={e => setAssignmentProject(e.target.value)}>
                        <option value="">Choose a project</option>
                        {ordered.filter(p => !p.archived && p.id !== temporaryAssignment.timer.projectId).map(p =>
                            <option key={p.id} value={p.id}>{p.number} - {projectPath(projects, p.id)}</option>)}
                    </select></label>
                    {!ordered.some(p => !p.archived && p.id !== temporaryAssignment.timer.projectId) && <p className="hint">No project is available. Cancel and create a project first.</p>}
                    {error && <p className="error" role="alert">{error}</p>}
                    <div className="dialog-actions"><button type="button" onClick={() => setTemporaryAssignment(null)}>Cancel</button><button className="primary" disabled={!assignmentProject}>Assign &amp; continue</button></div>
                </fieldset>
            </form>
        </Modal>}
        {timerEdit && <TimerStartForm initial={timerEdit} now={now} busy={busy} error={closeRequested ? '' : error} close={() => setTimerEdit(null)}
            save={start => run(() => UpdateTimerStart(timerEdit, start), () => setTimerEdit(null))}/>}
        {projectEdit && <ProjectForm initial={projectEdit} projects={ordered} definitions={state.propertyDefinitions} busy={busy} error={closeRequested ? '' : error} close={() => setProjectEdit(null)}
            save={p => run(() => SaveProject(p), () => setProjectEdit(null))}/>}
        {bookingEdit && <BookingForm initial={bookingEdit} projects={ordered} busy={busy} error={closeRequested ? '' : error} close={() => setBookingEdit(null)}
            save={b => run(() => SaveBooking(b), () => setBookingEdit(null))}
            remove={() => {setError(''); setConfirm({title: 'Delete booking?', message: 'This saved time booking will be permanently deleted.', action: () => DeleteBooking(bookingEdit.id)}); setBookingEdit(null);}}/>}
        {confirm && <Modal title={confirm.title} close={() => setConfirm(null)} busy={busy}>
            <p>{confirm.message}</p>{!closeRequested && error && <p className="error" role="alert">{error}</p>}
            <div className="dialog-actions"><button disabled={busy} onClick={() => setConfirm(null)}>Cancel</button><button disabled={busy} className="danger" onClick={() => void run(confirm.action, () => setConfirm(null))}>Confirm</button></div>
        </Modal>}
        {exportOpen && <Modal title="Export bookings to Excel" close={() => setExportOpen(false)} busy={busy}>
            <form onSubmit={e => {e.preventDefault(); void run(async () => {
                const path = await ExportBookings(exportFrom, exportTo);
                if (path) {setExportOpen(false); setNotice(`Excel export saved: ${path}`);}
            });}}><fieldset disabled={busy}>
                <div className="form-grid"><label>From date<input required type="date" value={exportFrom} onChange={e => setExportFrom(e.target.value)}/></label>
                    <label>Through date (inclusive)<input required type="date" min={exportFrom} value={exportTo} onChange={e => setExportTo(e.target.value)}/></label></div>
                <p className="hint">Exports saved bookings, including archived projects, with customer, project number, hierarchy, local start/end, hours, notes, color, and custom project properties. Bookings crossing the date boundaries are clipped to the selected range. The running timer is not included.</p>
                {!closeRequested && error && <p className="error" role="alert">{error}</p>}
                <div className="dialog-actions"><button type="button" onClick={() => setExportOpen(false)}>Cancel</button><button className="primary">Choose file &amp; export</button></div>
            </fieldset></form>
        </Modal>}
    </div>;
}

export default App;

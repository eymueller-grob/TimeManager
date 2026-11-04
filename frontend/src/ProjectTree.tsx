import {useId, useRef, useState, type ReactNode} from 'react';
import {ChevronDown, ChevronRight} from 'lucide-react';
import {type tracker} from '../wailsjs/go/models';
import {Button, ProjectMark} from './Controls';
import {Input} from './components/ui/input';
import {Popover, PopoverContent, PopoverTrigger} from './components/ui/popover';
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from './components/ui/collapsible';

type Project = tracker.Project;

export function projectPath(projects: Project[], id: string): string {
    const names: string[] = [];
    const seen = new Set<string>();
    while (id && !seen.has(id)) {
        seen.add(id);
        const project = projects.find(p => p.id === id);
        if (!project) break;
        names.unshift(project.name);
        id = project.parentId;
    }
    return names.join(' / ');
}

export function ProjectTree({projects, matches, searching = false, label, render}: {
    projects: Project[]; matches: Project[]; searching?: boolean; label: string;
    render: (project: Project, contextOnly: boolean) => ReactNode;
}) {
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
    const included = new Set(matches.map(p => p.id));
    const direct = new Set(included);
    for (const project of matches) {
        let parent = project.parentId;
        const seen = new Set([project.id]);
        while (parent && !seen.has(parent)) {
            seen.add(parent);
            const ancestor = projects.find(p => p.id === parent);
            if (!ancestor) break;
            included.add(parent);
            parent = ancestor.parentId;
        }
    }
    const visible = projects.filter(p => included.has(p.id));
    const ordered = [...visible].sort((a, b) => a.number.localeCompare(b.number, undefined, {numeric: true}) || a.name.localeCompare(b.name));
    const branch = (parent: string, visited: Set<string>): ReactNode => ordered.filter(p => p.parentId === parent || (!parent && !included.has(p.parentId))).map(project => {
        if (visited.has(project.id)) return null;
        const hasChildren = visible.some(p => p.parentId === project.id && !visited.has(p.id));
        const open = searching || !collapsed.has(project.id);
        return <Collapsible asChild key={project.id} open={open} onOpenChange={next => setCollapsed(previous => {
            const updated = new Set(previous);
            if (next) updated.delete(project.id); else updated.add(project.id);
            return updated;
        })}>
            <li className={direct.has(project.id) ? 'project-branch' : 'project-branch context-branch'}>
                <div className="tree-row">
                    {hasChildren ? <CollapsibleTrigger asChild><Button type="button" variant="ghost" size="icon"
                        aria-label={`${open ? 'Collapse' : 'Expand'} ${project.number} ${project.name}`} disabled={searching} title={searching ? 'Clear the search to collapse or expand projects.' : undefined}>
                        {open ? <ChevronDown/> : <ChevronRight/>}
                    </Button></CollapsibleTrigger> : <span className="tree-spacer"/>}
                    <div className="tree-row-content">{render(project, !direct.has(project.id))}</div>
                </div>
                {hasChildren && <CollapsibleContent><ul className="tree-children">{branch(project.id, new Set([...visited, project.id]))}</ul></CollapsibleContent>}
            </li>
        </Collapsible>;
    });
    return <ul className="project-tree" aria-label={label}>{branch('', new Set())}</ul>;
}

export function ProjectPicker({label, projects, eligible, value, onChange, required, autoFocus, emptyLabel = 'Select a project'}: {
    label: string; projects: Project[]; eligible: Project[]; value: string; onChange: (id: string) => void;
    required?: boolean; autoFocus?: boolean; emptyLabel?: string;
}) {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');
    const id = useId();
    const trigger = useRef<HTMLButtonElement>(null);
    const selected = projects.find(p => p.id === value);
    const query = search.trim().toLowerCase();
    const matches = eligible.filter(p => `${p.number} ${projectPath(projects, p.id)} ${p.customer}`.toLowerCase().includes(query));
    return <div className="project-picker">
        <label id={`${id}-label`} htmlFor={id}>{label}</label>
        <Popover open={open} onOpenChange={next => {setOpen(next); if (!next) setSearch('');}}>
            <PopoverTrigger asChild><Button id={id} ref={trigger} type="button" autoFocus={autoFocus} role="combobox"
                aria-labelledby={`${id}-label`} aria-controls={`${id}-choices`} aria-expanded={open} className="project-picker-trigger">
                <span>{selected && <ProjectMark project={selected}/>}{selected ? `${selected.number} - ${projectPath(projects, selected.id)}${selected.archived ? ' (archived)' : ''}` : emptyLabel}</span><ChevronDown/>
            </Button></PopoverTrigger>
            <PopoverContent id={`${id}-choices`} className="project-picker-content" align="start">
                <Input aria-label={`Search ${label.toLowerCase()}`} placeholder="Search projects…" value={search} onChange={event => setSearch(event.target.value)}/>
                <div className="project-picker-options">
                    {!required && <Button type="button" variant="ghost" onClick={() => {onChange(''); setOpen(false);}}>None (top-level project)</Button>}
                    <ProjectTree projects={projects} matches={matches} searching={!!query} label={`${label} hierarchy`}
                        render={(project, context) => context ? <span className="tree-context">{project.number} - {project.name}</span> :
                            <Button type="button" variant={project.id === value ? 'secondary' : 'ghost'} aria-pressed={project.id === value}
                                onClick={() => {onChange(project.id); setOpen(false); setSearch('');}}>
                                <ProjectMark project={project}/>{project.number} - {project.name}{project.archived ? ' (archived)' : ''}
                            </Button>}/>
                    {!matches.length && <p role="status" className="hint">No projects match.</p>}
                </div>
            </PopoverContent>
        </Popover>
        {required && <input className="project-picker-validation" aria-label={`${label} selection`} tabIndex={-1} value={value}
            required onChange={() => {}} onInvalid={event => {event.preventDefault(); trigger.current?.focus(); setOpen(true);}}/>}
    </div>;
}

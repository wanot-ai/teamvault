"use client";

import { useState, useMemo } from "react";
import { type Secret } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  Key,
} from "lucide-react";
import { cn } from "@/lib/utils";

// ─── Tree data structure ─────────────────────────────────────────────────────

interface TreeNode {
  name: string;
  path: string;
  isFolder: boolean;
  children: Map<string, TreeNode>;
  secret?: Secret;
}

function buildTree(secrets: Secret[]): TreeNode {
  const root: TreeNode = {
    name: "",
    path: "",
    isFolder: true,
    children: new Map(),
  };

  for (const secret of secrets) {
    const parts = secret.path.split("/");
    let current = root;

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isLast = i === parts.length - 1;
      const currentPath = parts.slice(0, i + 1).join("/");

      if (!current.children.has(part)) {
        current.children.set(part, {
          name: part,
          path: currentPath,
          isFolder: !isLast,
          children: new Map(),
          secret: isLast ? secret : undefined,
        });
      } else if (isLast) {
        // Update existing node with secret data
        const node = current.children.get(part)!;
        node.secret = secret;
        // If it was only a folder, it's now also a secret
        if (!node.isFolder) {
          node.isFolder = false;
        }
      } else {
        // Intermediate path — ensure it's a folder
        const node = current.children.get(part)!;
        node.isFolder = true;
      }

      current = current.children.get(part)!;
    }
  }

  return root;
}

function sortedChildren(node: TreeNode): TreeNode[] {
  return Array.from(node.children.values()).sort((a, b) => {
    // Folders first, then alphabetical
    if (a.isFolder && !a.secret && (!b.isFolder || b.secret)) return -1;
    if ((!a.isFolder || a.secret) && b.isFolder && !b.secret) return 1;
    return a.name.localeCompare(b.name);
  });
}

// ─── Tree Node Component ─────────────────────────────────────────────────────

interface TreeNodeComponentProps {
  node: TreeNode;
  depth: number;
  currentPath: string[];
  onNavigate: (path: string[]) => void;
  onSelectSecret: (secret: Secret) => void;
  expandedPaths: Set<string>;
  toggleExpand: (path: string) => void;
}

function TreeNodeComponent({
  node,
  depth,
  currentPath,
  onNavigate,
  onSelectSecret,
  expandedPaths,
  toggleExpand,
}: TreeNodeComponentProps) {
  const isExpanded = expandedPaths.has(node.path);
  const children = sortedChildren(node);
  const hasChildren = children.length > 0;
  const isFolder = node.isFolder && !node.secret;
  const isCurrent = currentPath.join("/") === node.path;

  const handleClick = () => {
    if (node.secret) {
      onSelectSecret(node.secret);
    } else if (isFolder && hasChildren) {
      toggleExpand(node.path);
    }
  };

  return (
    <div>
      <button
        onClick={handleClick}
        className={cn(
          "w-full flex items-center gap-2 px-2 py-1.5 rounded-md text-sm hover:bg-muted/50 transition-colors text-left group",
          isCurrent && "bg-primary/10 text-primary",
          node.secret && "cursor-pointer"
        )}
        style={{ paddingLeft: `${depth * 20 + 8}px` }}
      >
        {/* Expand/collapse chevron */}
        {isFolder && hasChildren ? (
          <span className="flex-shrink-0 w-4 h-4 flex items-center justify-center">
            {isExpanded ? (
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            ) : (
              <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
            )}
          </span>
        ) : (
          <span className="w-4" />
        )}

        {/* Icon */}
        {isFolder ? (
          isExpanded ? (
            <FolderOpen className="h-4 w-4 text-yellow-400 flex-shrink-0" />
          ) : (
            <Folder className="h-4 w-4 text-yellow-400 flex-shrink-0" />
          )
        ) : (
          <Key className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        )}

        {/* Name */}
        <span
          className={cn(
            "truncate",
            node.secret ? "font-mono text-xs" : "font-medium"
          )}
        >
          {node.name}
        </span>

        {/* Version badge for secrets */}
        {node.secret && (
          <Badge
            variant="secondary"
            className="ml-auto text-[10px] font-mono h-5 px-1.5 flex-shrink-0"
          >
            v{node.secret.current_version || 1}
          </Badge>
        )}
      </button>

      {/* Children */}
      {isExpanded && hasChildren && (
        <div>
          {children.map((child) => (
            <TreeNodeComponent
              key={child.path}
              node={child}
              depth={depth + 1}
              currentPath={currentPath}
              onNavigate={onNavigate}
              onSelectSecret={onSelectSecret}
              expandedPaths={expandedPaths}
              toggleExpand={toggleExpand}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// ─── Breadcrumb ──────────────────────────────────────────────────────────────

interface BreadcrumbProps {
  path: string[];
  onNavigate: (path: string[]) => void;
}

function SecretBreadcrumb({ path, onNavigate }: BreadcrumbProps) {
  if (path.length === 0) return null;

  return (
    <div className="flex items-center gap-1 text-sm overflow-x-auto pb-1">
      <button
        onClick={() => onNavigate([])}
        className="text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
      >
        <Folder className="h-4 w-4" />
      </button>
      {path.map((segment, idx) => (
        <div key={idx} className="flex items-center gap-1 flex-shrink-0">
          <span className="text-muted-foreground/50">/</span>
          <button
            onClick={() => onNavigate(path.slice(0, idx + 1))}
            className={cn(
              "hover:text-foreground transition-colors font-mono",
              idx === path.length - 1
                ? "text-foreground font-medium"
                : "text-muted-foreground"
            )}
          >
            {segment}
          </button>
        </div>
      ))}
    </div>
  );
}

// ─── Main SecretTree component ───────────────────────────────────────────────

interface SecretTreeProps {
  secrets: Secret[];
  onSelectSecret: (secret: Secret) => void;
}

export function SecretTree({ secrets, onSelectSecret }: SecretTreeProps) {
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [currentPath, setCurrentPath] = useState<string[]>([]);

  const tree = useMemo(() => buildTree(secrets), [secrets]);

  const toggleExpand = (path: string) => {
    setExpandedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const expandAll = () => {
    const allPaths = new Set<string>();
    const collect = (node: TreeNode) => {
      if (node.isFolder && node.children.size > 0) {
        allPaths.add(node.path);
        node.children.forEach((child) => collect(child));
      }
    };
    collect(tree);
    setExpandedPaths(allPaths);
  };

  const collapseAll = () => {
    setExpandedPaths(new Set());
  };

  const children = sortedChildren(tree);

  return (
    <div className="space-y-3">
      {/* Breadcrumb & controls */}
      <div className="flex items-center justify-between gap-4">
        <SecretBreadcrumb path={currentPath} onNavigate={setCurrentPath} />
        <div className="flex gap-1 flex-shrink-0">
          <Button
            variant="ghost"
            size="sm"
            className="text-xs h-7"
            onClick={expandAll}
          >
            Expand all
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-xs h-7"
            onClick={collapseAll}
          >
            Collapse
          </Button>
        </div>
      </div>

      {/* Tree */}
      <div className="border rounded-lg p-2 bg-card/50">
        {children.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <Folder className="h-10 w-10 text-muted-foreground/30 mb-2" />
            <p className="text-sm text-muted-foreground">No secrets in this path</p>
          </div>
        ) : (
          children.map((child) => (
            <TreeNodeComponent
              key={child.path}
              node={child}
              depth={0}
              currentPath={currentPath}
              onNavigate={setCurrentPath}
              onSelectSecret={onSelectSecret}
              expandedPaths={expandedPaths}
              toggleExpand={toggleExpand}
            />
          ))
        )}
      </div>
    </div>
  );
}

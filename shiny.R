library(shiny)
library(jsonlite)
library(RCurl)

sap_home <- Sys.getenv('SAP_HOME')
sap_app <- Sys.getenv('SAP_APP')
sap_port <- Sys.getenv('SAP_PORT')

base_uri <- Sys.getenv('BASE_URI')
auth_key <- Sys.getenv('AUTH_KEY')

print(sap_home)
print(sap_app)
print(sap_port)
print(base_uri)

#
Sys.setenv("PATH" = "/app/vendor/R/bin:/usr/local/bin:/usr/bin:/bin")

##

account <- Sys.getenv('ACCOUNT')
print(account)

content_uri <- paste0(base_uri,"/internal/fs/content?key=",auth_key,"&path=/home/",account)
list_uri <- paste0(base_uri,"/internal/fs/ls?key=",auth_key,"&path=/home/",account,"/data/",account,"/")

print(content_uri)
print(list_uri)

#
datafile_regexp <- paste0("^\\.\\./data/",account,"/.+\\.(dat|rda|csv)$")
account_home <- paste0("/home/",account)
print(datafile_regexp)
print(account_home)
#

file_list <- fromJSON(list_uri)$children
print(file_list)
print(account_home)

baseenv <- baseenv()
baseenv$datafile_regexp <- datafile_regexp
baseenv$content_uri <- content_uri
baseenv$file_list <- file_list
baseenv$account_home <- account_home

goboot_is_datafile <- function(path) {
    if (is.null(path) || is.na(path)) {
        return (FALSE)
    }
    env <- baseenv()
    re <- env$datafile_regexp
    m <- grepl(pattern = re, x = path)

    if (any(m)) {
        #../data/<goboot_app_id>/file.dat
        #../data/<goboot_app_id>/file.rda
        p <- paste0(env$account_home, substring(path, 3))
        b <- p %in% env$file_list[["path"]]
        print(paste("goboot_is_datafile: ", c(re, m, p, n, b), sep=" | "))
        return (b)
    }
    return (FALSE)
}

goboot_content_uri <- function(path) {
    env <- baseenv()
    re <- env$datafile_regexp
    filter <- ""
    if (!is.null(f <- get0("globalFilterObject"))) {
        json <- toJSON(f, pretty=TRUE)
        filter <- curlEscape(json)
    }
    u <- paste0(content_uri, substring(path, 3), "&filter=", filter, "&account=", account)
    print(paste("goboot_content_uri: ", c(re, u, filter), sep=" | "))
    return (u)
}
utils::assignInNamespace("goboot_is_datafile", goboot_is_datafile, "base")
utils::assignInNamespace("goboot_content_uri", goboot_content_uri, "base")

if (content_uri != "") {

## override base file functions and load from content uri
##
#file.create(..., showWarnings = TRUE)
#file_create <- function(..., showWarnings = TRUE) {
#    print(paste0("file.create: ", c(...)))
#    return (NA)
#}
#utils::assignInNamespace("file.create", file_create, "base")

#file.exists(...)
base_file_exists <- base::file.exists
file_exists <- function(...) {
    dots <- list(...)

    if (goboot_is_datafile(dots[[1]])) {
        print(paste0("file.exists: ",  c(...)))
        return (TRUE)
    }
    file._exists(...)
}
utils::assignInNamespace("file._exists", base_file_exists, "base")
utils::assignInNamespace("file.exists", file_exists, "base")

#file.remove(...)
#file_remove  <- function(...) {
#    print(paste0("file.remove: ", c(...)))
#    return (NA)
#}
#utils::assignInNamespace("file.remove", file_remove, "base")

#file.rename(from, to)
#file_rename  <- function(from, to) {
#    print(paste0("file.rename: ", c(from, to)))
#    return (NA)
#}
#utils::assignInNamespace("file.rename", file_rename, "base")

#file.append(file1, file2)
#file_append <- function(file1, file2) {
#    print(paste0("file.append: ", c(file1, file2)))
#    return (NA)
#}
#utils::assignInNamespace("file.append", file_append, "base")

#file.copy(from, to, overwrite = recursive, recursive = FALSE, copy.mode = TRUE, copy.date = FALSE)
#file_copy <- function(from, to, overwrite = recursive, recursive = FALSE, copy.mode = TRUE, copy.date = FALSE) {
#    print(paste0("file.copy: ", c(from, to, overwrite, recursive, copy.mode, copy.date)))
#    return (NA)
#}
#utils::assignInNamespace("file.copy", file_copy, "base")

#file.symlink(from, to)
#file_symlink <- function(file1, file2) {
#    print(paste0("file.symlink: ", c(file1, file2)))
#    return (NA)
#}
#utils::assignInNamespace("file.symlink", file_symlink, "base")

#file.link(from, to)
#file_link <- function(file1, file2) {
#    print(paste0("file.link: ", c(file1, file2)))
#    return (NA)
#}
#utils::assignInNamespace("file.link", file_link, "base")

#file.info(..., extra_cols = TRUE)
#base_file_info <- base::file.info
#file_info <- function (..., extra_cols = TRUE) {
#    dots <- list(...)
#    n <- dots[1]
#print(paste0("file.info: ", n))
#    if (goboot_is_datafile(n)) {
#        print(paste0("file.info: ", c(...)))
#        return (NA)
#    }
#    file._info(c(...), extra_cols)
#}
#utils::assignInNamespace("file._info", base_file_info, "base")
#utils::assignInNamespace("file.info", file_info, "base")

##
#load(file, envir = parent.frame(), verbose = FALSE)
base_load <- load
xxx_load <- function (file, envir = parent.frame(), verbose = FALSE) {
    env <- baseenv()
    if (is.null(env$content_uri)) {
        base_load(file, envir, verbose)
    } else {
        print(paste0("loading from uri: ", file))
#        base_load(url(goboot_content_uri(file)), envir, verbose)
        getBinaryURL(goboot_content_uri(file))
    }
}
utils::assignInNamespace("base_load", base_load, "base")
utils::assignInNamespace("load", xxx_load, "base")

##
}

setwd(paste0(sap_home,"/", sap_app))
shiny::runApp("app", host = '0.0.0.0', port = as.numeric(sap_port), launch.browser = FALSE)
##
